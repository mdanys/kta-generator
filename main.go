package main

import (
	"context"
	"fmt"
	"log"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/redis/go-redis/v9"
)

func main() {
	awsConfig, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("load AWS configuration: %v", err)
	}
	s3Client := s3.NewFromConfig(awsConfig)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     envOrDefault("REDIS_ADDR", "localhost:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
	})
	defer redisClient.Close()

	bucket := os.Getenv("AWS_S3_BUCKET")
	if bucket == "" {
		log.Fatal("AWS_S3_BUCKET must be set")
	}

	// Example 1: blank card (name area empty, ready for handwriting)
	blank := KTAData{
		NomorKTA:         "8831740500000001",
		Kecamatan:        "Kecamatan Kelapa Gading",
		Kota:             "KOTA JAKARTA UTARA",
		Provinsi:         "DKI Jakarta",
		LogoS3Bucket:     bucket,
		LogoS3Key:        "assets/logo.png",
		FontS3Bucket:     bucket,
		FontRegularS3Key: "assets/fonts/Roboto-Regular.ttf",
		FontBoldS3Key:    "assets/fonts/Roboto-Bold.ttf",
		FontBlackS3Key:   "assets/fonts/Roboto-Black.ttf",
		FontItalicS3Key:  "assets/fonts/Roboto-Italic.ttf",
		S3Bucket:         bucket,
		S3Key:            "kta/output_blank.png",
	}
	if err := GenerateKTACard(s3Client, redisClient, blank); err != nil {
		log.Fatalf("failed to generate blank card: %v", err)
	}
	fmt.Printf("✓ Uploaded: s3://%s/%s\n", blank.S3Bucket, blank.S3Key)

	// Example 2: filled card (name already written)
	filled := KTAData{
		NomorKTA:         "8831740500000002",
		NamaAnggota:      "Siti Rahmawati",
		Kecamatan:        "Kecamatan Kelapa Gading",
		Kota:             "KOTA JAKARTA UTARA",
		Provinsi:         "DKI Jakarta",
		LogoS3Bucket:     bucket,
		LogoS3Key:        "assets/logo.png",
		FontS3Bucket:     bucket,
		FontRegularS3Key: "assets/fonts/Roboto-Regular.ttf",
		FontBoldS3Key:    "assets/fonts/Roboto-Bold.ttf",
		FontBlackS3Key:   "assets/fonts/Roboto-Black.ttf",
		FontItalicS3Key:  "assets/fonts/Roboto-Italic.ttf",
		S3Bucket:         bucket,
		S3Key:            "kta/output_filled.png",
	}
	if err := GenerateKTACard(s3Client, redisClient, filled); err != nil {
		log.Fatalf("failed to generate filled card: %v", err)
	}
	fmt.Printf("✓ Uploaded: s3://%s/%s\n", filled.S3Bucket, filled.S3Key)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
