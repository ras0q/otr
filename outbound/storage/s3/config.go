package s3

// Config configures an S3-compatible object store.
type Config struct {
	Bucket   string
	Region   string
	Endpoint string // optional; set for MinIO, R2, etc.
	Prefix   string // optional key prefix, e.g. otr/prod
	// UsePathStyle forces path-style URLs (common for MinIO).
	UsePathStyle bool
	AccessKeyID  string
	SecretKey    string
}
