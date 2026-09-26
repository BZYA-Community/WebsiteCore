//go:build constraint

package storage

import (
	"github.com/BZYA-Community/WebsiteCore/internal/core"
)

var (
	_ core.ObjectStorageService = (*aliossServant)(nil)
	_ core.OssCreateService     = (*aliossCreateServant)(nil)
	_ core.OssCreateService     = (*aliossCreateRetentionServant)(nil)
	_ core.OssCreateService     = (*aliossCreateTempDirServant)(nil)
	_ core.VersionInfo          = (*aliossServant)(nil)

	_ core.ObjectStorageService = (*localossServant)(nil)
	_ core.OssCreateService     = (*localossCreateServant)(nil)
	_ core.OssCreateService     = (*localossCreateTempDirServant)(nil)
	_ core.VersionInfo          = (*localossServant)(nil)
)
