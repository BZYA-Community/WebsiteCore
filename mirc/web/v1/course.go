package v1

import (
	. "github.com/alimy/mir/v5"

	"github.com/BZYA-Community/WebsiteCore/internal/model/web"
)

// CourseLoose 课程公开服务(游客可读, 登录用户携带身份用于评论可见性)
type CourseLoose struct {
	Schema `mir:"v1,chain"`

	// CourseGroups 课程分组列表(含各组课程数)
	CourseGroups func(Get, web.CourseGroupsReq) web.CourseGroupsResp `mir:"course/groups"`
	// CourseList 课程列表: group_id过滤分组, keyword搜索标题/简介
	CourseList func(Get, web.CourseListReq) web.CourseListResp `mir:"course/list"`
	// CourseDetail 课程详情
	CourseDetail func(Get, web.CourseDetailReq) web.CourseDetailResp `mir:"course"`
	// CourseComments 课程评论列表(审核可见性与帖子评论同口径)
	CourseComments func(Get, web.CourseCommentsReq) web.CourseCommentsResp `mir:"course/comments"`
	// CourseVideo 课程视频签名播放地址
	CourseVideo func(Get, web.CourseVideoReq) web.CourseVideoResp `mir:"course/video"`
	// CoursePlay 播放计数(每次播放+1)
	CoursePlay func(Post, web.CoursePlayReq) web.CoursePlayResp `mir:"course/play"`
}

// CoursePriv 课程登录用户服务(评论楼中楼)
type CoursePriv struct {
	Schema `mir:"v1,chain"`

	// CreateCourseComment 发布课程评论(无角色用户进入审核)
	CreateCourseComment func(Post, web.CreateCourseCommentReq) web.CreateCourseCommentResp `mir:"course/comment"`
	// DeleteCourseComment 删除课程评论(本人或管理员)
	DeleteCourseComment func(Delete, web.DeleteCourseCommentReq) `mir:"course/comment"`
	// CreateCourseCommentReply 回复课程评论(无角色用户进入审核)
	CreateCourseCommentReply func(Post, web.CreateCourseCommentReplyReq) web.CreateCourseCommentReplyResp `mir:"course/comment/reply"`
	// DeleteCourseCommentReply 删除课程回复(本人或管理员)
	DeleteCourseCommentReply func(Delete, web.DeleteCourseCommentReplyReq) `mir:"course/comment/reply"`
}

// CourseAdmin 课程管理服务(管理员/运维)
type CourseAdmin struct {
	Schema `mir:"v1,chain"`

	// CreateCourseGroup 创建课程分组
	CreateCourseGroup func(Post, web.CourseGroupReq) web.CourseGroupResp `mir:"admin/course/group"`
	// UpdateCourseGroup 更新课程分组
	UpdateCourseGroup func(Post, web.UpdateCourseGroupReq) `mir:"admin/course/group/update"`
	// DeleteCourseGroup 删除课程分组(分组下存在课程时拒绝)
	DeleteCourseGroup func(Post, web.DeleteCourseGroupReq) `mir:"admin/course/group/delete"`
	// CreateCourse 创建课程(创建即上线)
	CreateCourse func(Post, web.CreateCourseReq) web.CreateCourseResp `mir:"admin/course"`
	// UpdateCourse 更新课程
	UpdateCourse func(Post, web.UpdateCourseReq) `mir:"admin/course/update"`
	// DeleteCourse 删除课程(硬删除 连带评论与OSS对象)
	DeleteCourse func(Post, web.DeleteCourseReq) `mir:"admin/course/delete"`
	// CourseUploadCredential 获取视频上传凭证(AliOSS直传policy/其他OSS返回proxy)
	CourseUploadCredential func(Get, web.CourseUploadCredentialReq) web.CourseUploadCredentialResp `mir:"admin/course/upload-credential"`
	// UploadCourseVideo 代理模式上传课程视频(multipart)
	UploadCourseVideo func(Post, web.UploadCourseVideoReq) web.UploadCourseVideoResp `mir:"admin/course/video"`
}
