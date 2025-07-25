package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<30)
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	videoMeta, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video doesn't exist", err)
		return
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}
	if videoMeta.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not the owner of the video", err)
		return
	}

	file, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse the video", err)
		return
	}
	defer file.Close()

	fileType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if fileType != "video/mp4" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Wrong media type", err)
		return
	}
	tempFile, err := os.CreateTemp("/Users/nikolatosic/Desktop/Personal", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video", err)
		return
	}
	tempFile.Seek(0, io.SeekStart)

	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something failed try again", err)
		return
	}

	ratio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get ratio", err)
		return
	}
	proccesedVideoPath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		fmt.Println(err, "errorfdsfsfdgsd")
		respondWithError(w, http.StatusInternalServerError, "Couldn't process the video", err)
		return
	}
	proccesedVideo, err := os.Open(proccesedVideoPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't open the processed video", err)
		return
	}

	var prefix string
	switch ratio {
	case "16:9":
		prefix = "landscape"
	case "9:16":
		prefix = "portrait"
	default:
		prefix = "other"
	}
	randomPath := fmt.Sprintf("%v/%v.mp4", prefix, base64.RawURLEncoding.EncodeToString(b))

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(randomPath),
		Body:        proccesedVideo,
		ContentType: aws.String(fileType),
	})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload to S3", err)
		return
	}

	videoUrl := fmt.Sprintf("https://%v/%v", cfg.s3CfDistribution, randomPath)

	videoMeta.VideoURL = &videoUrl

	err = cfg.db.UpdateVideo(videoMeta)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update the video to DB", err)
		return
	}
	respondWithJSON(w, http.StatusCreated, videoMeta)
}
