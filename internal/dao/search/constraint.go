//go:build constraint

package search

import "github.com/BZYA-Community/WebsiteCore/internal/core"

var (
	_ core.TweetSearchService = (*bridgeTweetSearchServant)(nil)

	_ core.TweetSearchService = (*meiliTweetSearchServant)(nil)
	_ core.VersionInfo        = (*meiliTweetSearchServant)(nil)

	_ core.TweetSearchService = (*sqlTweetSearchServant)(nil)
	_ core.VersionInfo        = (*sqlTweetSearchServant)(nil)
)
