package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/redis/go-redis/v9"
)

const s3AssetCacheTTL = 24 * time.Hour

func loadS3Asset(ctx context.Context, cache *redis.Client, client *s3.Client, bucket, key string) ([]byte, error) {
	cacheIdentity := sha256.Sum256([]byte(bucket + "\x00" + key))
	cacheKey := "kta-generator:s3-asset:" + hex.EncodeToString(cacheIdentity[:])

	data, err := cache.Get(ctx, cacheKey).Bytes()
	if err == nil {
		return data, nil
	}
	if err != redis.Nil {
		log.Printf("warning: Redis asset cache read failed: %v", err)
	}

	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get s3://%s/%s: %w", bucket, key, err)
	}
	defer result.Body.Close()

	data, err = io.ReadAll(result.Body)
	if err != nil {
		return nil, fmt.Errorf("read s3://%s/%s: %w", bucket, key, err)
	}
	if err := cache.Set(ctx, cacheKey, data, s3AssetCacheTTL).Err(); err != nil {
		log.Printf("warning: Redis asset cache write failed: %v", err)
	}

	return data, nil
}
