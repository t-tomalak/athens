package s3

import (
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/gomods/athens/pkg/config"
	"github.com/gomods/athens/pkg/errors"
)

// Storage implements (./pkg/storage).Backend and
// also provides a function to fetch the location of a module
// Storage uses amazon aws go SDK which expects these env variables
// - AWS_REGION 			- region for this storage, e.g 'us-west-2'
// - AWS_ACCESS_KEY_ID		-
// - AWS_SECRET_ACCESS_KEY 	-
// - AWS_SESSION_TOKEN		- [optional]
// For information how to get your keyId and access key turn to official aws docs: https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/setting-up.html
type Storage struct {
	bucket   string
	baseURI  *url.URL
	uploader s3manageriface.UploaderAPI
	s3API    s3iface.S3API
	s3Conf   *config.S3Config
}

// New creates a new AWS S3 CDN saver
func New(s3Conf *config.S3Config, options ...func(*aws.Config)) (*Storage, error) {
	const op errors.Op = "s3.New"
	u, err := url.Parse(fmt.Sprintf("https://%s.s3.amazonaws.com", s3Conf.Bucket))
	if err != nil {
		return nil, errors.E(op, err)
	}

	awsConfig := &aws.Config{
		Credentials: credentials.NewStaticCredentials(s3Conf.Key, s3Conf.Secret, s3Conf.Token),
		Region:      aws.String(s3Conf.Region),
	}

	for _, o := range options {
		o(awsConfig)
	}

	// create a session
	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, errors.E(op, err)
	}
	uploader := s3manager.NewUploader(sess)

	return &Storage{
		bucket:   s3Conf.Bucket,
		uploader: uploader,
		s3API:    uploader.S3,
		baseURI:  u,
		s3Conf:   s3Conf,
	}, nil
}
