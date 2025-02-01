package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
	bucketName := "cloudfuse3"
	prefixPath := "COL-L465803D002_testing_size_tracker_1/"
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		fmt.Println(err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://s3.us-east-1.sv15.lyve.seagate.com")
	})
	var totalSize int64
	var continuationToken *string
	for {
		resp, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucketName),
			Prefix:            aws.String(prefixPath),
			ContinuationToken: continuationToken,
		})

		if err != nil {
			fmt.Println(err)
		}

		for _, obj := range resp.Contents {
			totalSize += *obj.Size
		}

		if !*resp.IsTruncated {
			break
		}

		continuationToken = resp.NextContinuationToken
	}

	fmt.Println("Total size is: ", totalSize)
}
