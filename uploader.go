package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	log "github.com/sirupsen/logrus"
	//"github.com/aws/aws-sdk-go/aws/awserr"
	"bytes"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jinzhu/gorm"
	"strconv"
)

func startUploader(channel chan ImageUploadTask, config S3Config) {

	sess, err := session.NewSession(
		&aws.Config{
			Region: aws.String(config.Region),
			Credentials: credentials.NewStaticCredentials(
				config.AccessKey,
				config.SecretKey,
				"",
			),
		},
	)
	if err != nil {
		panic(err)
	}

	svc := s3.New(sess)

	for {
		select {
		case task := <-channel:
			log.Print("[Event] New Image to Upload ", len(task.Buffer), " ", task.Path)

			byteReader := bytes.NewReader(task.Buffer)

			_, err := svc.PutObject(
				&s3.PutObjectInput{
					Bucket:        aws.String(config.Bucket),
					Key:           &task.Path,
					Body:          byteReader,
					ContentLength: aws.Int64(byteReader.Size()),
					ContentType:   aws.String("image/jpeg"),
					ACL:           aws.String(s3.ObjectCannedACLPublicRead),
				},
			)
			if err != nil {
				panic(err)
			}
		}
	}
}

func startStatusUploader(status chan VideoUploadTask, config S3Config, DB *gorm.DB) {
	for {
		select {
		case task := <-status:
            if (task.VideoId > 0) {
                DB.Exec("UPDATE videos SET status = 'active' WHERE id = ?", task.VideoId)
                log.Print("updated video status " + strconv.FormatUint(task.VideoId, 10))
            }
		}
	}
}

func startVideoUploader(channel chan VideoUploadTask, status chan VideoUploadTask, config S3Config) {

	sess, err := session.NewSession(
		&aws.Config{
			Region: aws.String(config.Region),
			Credentials: credentials.NewStaticCredentials(
				config.AccessKey,
				config.SecretKey,
				"",
			),
		},
	)
	if err != nil {
		panic(err)
	}

	svc := s3.New(sess)

	for {
		select {
		case task := <-channel:
			log.Print("[Event] New video to Upload ", len(task.Buffer), " ", task.Path)

			byteReader := bytes.NewReader(task.Buffer)

			_, err := svc.PutObject(
				&s3.PutObjectInput{
					Bucket:        aws.String(config.Bucket),
					Key:           &task.Path,
					Body:          byteReader,
					ContentLength: aws.Int64(byteReader.Size()),
					ContentType:   aws.String("video/mp4"),
					ACL:           aws.String(s3.ObjectCannedACLPublicRead),
				},
			)

			if err != nil {
				panic(err)
			} else {
				status <- task
			}
		}
	}
}

func startAudioUploader(channel chan AudioUploadTask, config S3Config) {

	sess, err := session.NewSession(
		&aws.Config{
			Region: aws.String(config.Region),
			Credentials: credentials.NewStaticCredentials(
				config.AccessKey,
				config.SecretKey,
				"",
			),
		},
	)
	if err != nil {
		panic(err)
	}

	svc := s3.New(sess)

	for {
		select {
		case task := <-channel:
			log.Print("[Event] New audio to Upload ", len(task.Buffer), " ", task.Path)

			byteReader := bytes.NewReader(task.Buffer)

			_, err := svc.PutObject(
				&s3.PutObjectInput{
					Bucket:        aws.String(config.Bucket),
					Key:           &task.Path,
					Body:          byteReader,
					ContentLength: aws.Int64(byteReader.Size()),
					ContentType:   aws.String("audio/mpeg"),
					ACL:           aws.String(s3.ObjectCannedACLPublicRead),
				},
			)

			if err != nil {
				panic(err)
			}
		}
	}
}
