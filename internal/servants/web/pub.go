// Copyright 2022 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"image/color"
	"image/png"
	"regexp"
	"unicode/utf8"

	api "github.com/BZYA-Community/WebsiteCore/auto/api/v1"
	"github.com/BZYA-Community/WebsiteCore/internal/core/ms"
	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/base"
	"github.com/BZYA-Community/WebsiteCore/internal/servants/web/assets"
	"github.com/BZYA-Community/WebsiteCore/pkg/app"
	"github.com/BZYA-Community/WebsiteCore/pkg/utils"
	"github.com/BZYA-Community/WebsiteCore/pkg/version"
	"github.com/BZYA-Community/WebsiteCore/pkg/xerror"
	"github.com/afocus/captcha"
	"github.com/gofrs/uuid/v5"
	"github.com/sirupsen/logrus"
)

const (
	// _MaxAccountLoginErrTimes 账号维度次要上限(#28兜底): 复合键(账号×IP, 由
	// LoginLockout 中间件按 10次/IP 控制)挡单来源IP, 此阈值抬高后用于兜底
	// 分布式撞库; 1小时内全局失败达该次数才锁定账号(等待1小时自动解锁)。
	_MaxAccountLoginErrTimes = 50
	_MaxPhoneCaptcha         = 10
)

// _dummyPasswordHash 用户不存在时用于等时密码比较的占位哈希(#28 防时序枚举)
var _dummyPasswordHash = utils.HashPassword("WebsiteCore-timing-equalizer")

type pubSrv struct {
	api.UnimplementedPubServant
	*base.DaoServant
}

func (s *pubSrv) SendCaptcha(req *web.SendCaptchaReq) error {
	ctx := context.Background()

	// 验证图片验证码
	if captcha, err := s.Redis.GetImgCaptcha(ctx, req.ImgCaptchaID); err != nil || string(captcha) != req.ImgCaptcha {
		logrus.Debugf("get captcha err:%s expect:%s got:%s", err, captcha, req.ImgCaptcha)
		return web.ErrErrorCaptchaPassword
	}
	s.Redis.DelImgCaptcha(ctx, req.ImgCaptchaID)

	// 今日频次限制
	if count, _ := s.Redis.GetCountSmsCaptcha(ctx, req.Phone); count >= _MaxPhoneCaptcha {
		return web.ErrTooManyPhoneCaptchaSend
	}

	if err := s.Ds.SendPhoneCaptcha(req.Phone); err != nil {
		return xerror.ServerError
	}
	// 写入计数缓存
	s.Redis.IncrCountSmsCaptcha(ctx, req.Phone)

	return nil
}

func (s *pubSrv) GetCaptcha() (*web.GetCaptchaResp, error) {
	cap := captcha.New()
	if err := cap.AddFontFromBytes(assets.ComicBytes); err != nil {
		logrus.Errorf("cap.AddFontFromBytes err:%s", err)
		return nil, xerror.ServerError
	}
	cap.SetSize(160, 64)
	cap.SetDisturbance(captcha.MEDIUM)
	cap.SetFrontColor(color.RGBA{0, 0, 0, 255})
	cap.SetBkgColor(color.RGBA{218, 240, 228, 255})
	img, password := cap.Create(6, captcha.NUM)
	emptyBuff := bytes.NewBuffer(nil)
	if err := png.Encode(emptyBuff, img); err != nil {
		logrus.Errorf("png.Encode err:%s", err)
		return nil, xerror.ServerError
	}
	key := utils.EncodeMD5(uuid.Must(uuid.NewV4()).String())
	// 五分钟有效期
	s.Redis.SetImgCaptcha(context.Background(), key, password)
	return &web.GetCaptchaResp{
		Id:      key,
		Content: "data:image/png;base64," + base64.StdEncoding.EncodeToString(emptyBuff.Bytes()),
	}, nil
}

func (s *pubSrv) Register(req *web.RegisterReq) (*web.RegisterResp, error) {
	if _disallowUserRegister {
		return nil, web.ErrDisallowUserRegister
	}
	// 用户名检查
	if err := s.validUsername(req.Username); err != nil {
		return nil, err
	}
	// 密码检查
	if err := checkPassword(req.Password); err != nil {
		logrus.Errorf("scheckPassword err: %v", err)
		return nil, web.ErrUserRegisterFailed
	}
	password, salt := encryptPasswordAndSalt(req.Password)
	user := &ms.User{
		Nickname: req.Username,
		Username: req.Username,
		Password: password,
		Avatar:   getRandomAvatar(),
		Salt:     salt,
		Status:   ms.UserStatusNormal,
	}
	user, err := s.Ds.CreateUser(user)
	if err != nil {
		logrus.Errorf("Ds.CreateUser err: %s", err)
		return nil, web.ErrUserRegisterFailed
	}
	return &web.RegisterResp{
		UserId:   user.ID,
		Username: user.Username,
	}, nil
}

func (s *pubSrv) Login(req *web.LoginReq) (*web.LoginResp, error) {
	ctx := context.Background()
	user, err := s.Ds.GetUserByUsername(req.Username)
	if err != nil {
		logrus.Errorf("Ds.GetUserByUsername err:%s", err)
	}
	if err != nil || user.Model == nil || user.ID <= 0 {
		// 凭据类失败统一返回同一业务码与文案(#28 防用户名枚举):
		// "用户不存在"与"密码错误"不再可区分;
		// 用户不存在时补一次等价的密码比较, 避免时序差异泄露账号是否存在
		validPassword(_dummyPasswordHash, req.Password)
		return nil, xerror.UnauthorizedAuthFailed
	}

	// 账号维度次要上限(#28): 复合键(账号×IP)锁定由 LoginLockout 中间件负责,
	// 此处更高阈值仅用于兜底分布式撞库
	if count, err := s.Redis.GetCountLoginErr(ctx, user.ID); err == nil && count >= _MaxAccountLoginErrTimes {
		return nil, web.ErrTooManyLoginError
	}
	// 对比密码是否正确
	if !validPassword(user.Password, req.Password) {
		// 登录错误计数
		s.Redis.IncrCountLoginErr(ctx, user.ID)
		return nil, xerror.UnauthorizedAuthFailed
	}
	// 封停提示仅在密码正确后返回(#28): 密码持有者可见, 不构成枚举侧信道
	if user.Status == ms.UserStatusClosed {
		return nil, web.ErrUserHasBeenBanned
	}
	// 清空登录计数
	s.Redis.DelCountLoginErr(ctx, user.ID)

	token, err := app.GenerateToken(user)
	if err != nil {
		logrus.Errorf("app.GenerateToken err: %v", err)
		return nil, xerror.UnauthorizedTokenGenerate
	}
	return &web.LoginResp{
		Token: token,
	}, nil
}

func (s *pubSrv) Version() (*web.VersionResp, error) {
	return &web.VersionResp{
		BuildInfo: version.ReadBuildInfo(),
	}, nil
}

// validUsername 验证用户
func (s *pubSrv) validUsername(username string) error {
	// 检测用户是否合规
	if utf8.RuneCountInString(username) < 3 || utf8.RuneCountInString(username) > 12 {
		return web.ErrUsernameLengthLimit
	}

	if !regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(username) {
		return web.ErrUsernameCharLimit
	}

	// 重复检查
	user, _ := s.Ds.GetUserByUsername(username)
	if user.Model != nil && user.ID > 0 {
		return web.ErrUsernameHasExisted
	}
	return nil
}

func newPubSrv(s *base.DaoServant) api.Pub {
	return &pubSrv{
		DaoServant: s,
	}
}
