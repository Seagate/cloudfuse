/*
 the following code below was copied and pasted from the following sources:

https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/endpoints/

https://docs.aws.amazon.com/code-library/latest/ug/go_2_s3_code_examples.html

Since the package is main and runs a main(), you'll need to copy this to a separate directory outside of the project to run the example.
*/

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

/*
descritpion:
	sample code to connect to LyveCloud S3 and output bucket names.

input:
	N/A

output:
	N/A return values. prints to screen listing up to 10 buckets in the S3.
*/

func main() {
	// Load the Shared AWS Configuration (~/.aws/config)

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == s3.ServiceID && region == "us-east-1" {
			return aws.Endpoint{
				PartitionID:   "aws",
				URL:           "https://s3.us-east-1.lyvecloud.seagate.com",
				SigningRegion: "us-east-1",
			}, nil
		}
		return aws.Endpoint{}, fmt.Errorf("unknown endpoint requested")
	})

	//awsEndoingResolvedOpetions := aws.
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithEndpointResolverWithOptions(customResolver))

	if err != nil {
		log.Fatal(err)
	}

	// Create an Amazon S3 service client
	s3Client := s3.NewFromConfig(cfg)
	count := 10
	fmt.Printf("Let's list up to %v buckets for your account.\n", count)
	result, err := s3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		fmt.Printf("Couldn't list buckets for your account. Here's why: %v\n", err)
		return
	}
	if len(result.Buckets) == 0 {
		fmt.Println("You don't have any buckets!")
	} else {
		if count > len(result.Buckets) {
			count = len(result.Buckets)
		}
		for _, bucket := range result.Buckets[:count] {
			fmt.Printf("\t%v\n", *bucket.Name)
		}
	}
}
