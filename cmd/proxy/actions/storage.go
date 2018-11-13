package actions

import (
	"context"
	"fmt"

	"github.com/gomods/athens/pkg/config"
	"github.com/gomods/athens/pkg/errors"
	"github.com/gomods/athens/pkg/storage"
	"github.com/gomods/athens/pkg/storage/fs"
	"github.com/gomods/athens/pkg/storage/gcp"
	"github.com/gomods/athens/pkg/storage/mem"
	"github.com/gomods/athens/pkg/storage/minio"
	"github.com/gomods/athens/pkg/storage/mongo"
	"github.com/gomods/athens/pkg/storage/s3"
	"github.com/spf13/afero"
)

// GetStorage returns storage backend based on env configuration
func GetStorage(storageType string, storageConfig *config.StorageConfig) (storage.Backend, error) {
	const op errors.Op = "actions.GetStorage"
	switch storageType {
	case "memory":
		return mem.NewStorage()
	case "mongo":
		if storageConfig.Mongo == nil {
			return nil, errors.E(op, "Invalid Mongo Storage Configuration")
		}
		return mongo.NewStorage(storageConfig.Mongo)
	case "disk":
		if storageConfig.Disk == nil {
			return nil, errors.E(op, "Invalid Disk Storage Configuration")
		}
		rootLocation := storageConfig.Disk.RootPath
		s, err := fs.NewStorage(rootLocation, afero.NewOsFs())
		if err != nil {
			errStr := fmt.Sprintf("could not create new storage from os fs (%s)", err)
			return nil, errors.E(op, errStr)
		}
		return s, nil
	case "minio":
		if storageConfig.Minio == nil {
			return nil, errors.E(op, "Invalid Minio Storage Configuration")
		}
		return minio.NewStorage(storageConfig.Minio)
	case "gcp":
		if storageConfig.GCP == nil {
			return nil, errors.E(op, "Invalid GCP Storage Configuration")
		}
		return gcp.New(context.Background(), storageConfig.GCP)
	case "s3":
		if storageConfig.S3 == nil {
			return nil, errors.E(op, "Invalid S3 Storage Configuration")
		}
		return s3.New(storageConfig.S3)
	default:
		return nil, fmt.Errorf("storage type %s is unknown", storageType)
	}
}
