package config

// StorageConfig provides configs for various storage backends
type StorageConfig struct {
	Disk  *DiskConfig
	GCP   *GCPConfig
	Minio *MinioConfig
	Mongo *MongoConfig
	S3    *S3Config
	Azure *AzureConfig
}

func setStorageTimeouts(s *StorageConfig, defaultTimeout int) {
	if s == nil {
		return
	}
	if s.GCP != nil && s.GCP.Timeout == 0 {
		s.GCP.Timeout = defaultTimeout
	}
	if s.Minio != nil && s.Minio.Timeout == 0 {
		s.Minio.Timeout = defaultTimeout
	}
	if s.Mongo != nil && s.Mongo.Timeout == 0 {
		s.Mongo.Timeout = defaultTimeout
	}
	if s.S3 != nil && s.S3.Timeout == 0 {
		s.S3.Timeout = defaultTimeout
	}
	if s.Azure != nil && s.Azure.Timeout == 0 {
		s.Azure.Timeout = defaultTimeout
	}
}
