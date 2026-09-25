// Copyright 2023 ROC. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package ms

const (
	ActRegisterUser act = iota
	ActCreatePublicTweet
	ActCreatePublicAttachment
	ActCreatePublicPicture
	ActCreatePublicVideo
	ActCreatePrivateTweet
	ActCreatePrivateAttachment
	ActCreatePrivatePicture
	ActCreatePrivateVideo
	ActCreateFriendTweet
	ActCreateFriendAttachment
	ActCreateFriendPicture
	ActCreateFriendVideo
	ActCreatePublicComment
	ActCreatePublicPicureComment
	ActCreateFriendComment
	ActCreateFriendPicureComment
	ActCreatePrivateComment
	ActCreatePrivatePicureComment
	ActStickTweet
	ActTopTweet
	ActLockTweet
	ActVisibleTweet
	ActDeleteTweet
	ActCreateActivationCode
)

type (
	act uint8

	Action struct {
		Act    act
		UserId int64
	}
)

// IsAllow default true if user is admin
func (a act) IsAllow(user *User, userId int64, isFriend bool, isActivation bool) bool {
	if user.IsAdmin {
		return true
	}
	if user.ID == userId && isActivation {
		switch a {
		case ActCreatePublicTweet,
			ActCreatePublicAttachment,
			ActCreatePublicPicture,
			ActCreatePublicVideo,
			ActCreatePrivateTweet,
			ActCreatePrivateAttachment,
			ActCreatePrivatePicture,
			ActCreatePrivateVideo,
			ActCreateFriendTweet,
			ActCreateFriendAttachment,
			ActCreateFriendPicture,
			ActCreateFriendVideo,
			ActCreatePrivateComment,
			ActCreatePrivatePicureComment,
			ActStickTweet,
			ActLockTweet,
			ActVisibleTweet,
			ActDeleteTweet:
			return true
		}
	}

	if user.ID == userId && !isActivation {
		switch a {
		case ActCreatePrivateTweet,
			ActCreatePrivateComment,
			ActStickTweet,
			ActLockTweet,
			ActDeleteTweet:
			return true
		}
	}

	if isFriend && isActivation {
		switch a {
		case ActCreatePublicComment,
			ActCreatePublicPicureComment,
			ActCreateFriendComment,
			ActCreateFriendPicureComment:
			return true
		}
	}

	if !isFriend && isActivation {
		switch a {
		case ActCreatePublicComment,
			ActCreatePublicPicureComment:
			return true
		}
	}

	return false
}
