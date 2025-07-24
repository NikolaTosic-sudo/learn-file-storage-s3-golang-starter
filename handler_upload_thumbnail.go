package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

type Video struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ThumbnailURL *string   `json:"thumbnail_url"`
	VideoURL     *string   `json:"video_url"`
}

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse", err)
		return
	}

	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't form file", err)
		return
	}
	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse media type", err)
		return
	}

	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Wrong media type", err)
		return
	}

	videoMeta, err := cfg.db.GetVideo(videoID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Video not found", err)
		return
	}

	if videoMeta.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User is not the owner", err)
		return
	}

	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something failed try again", err)
		return
	}
	randomPath := base64.RawURLEncoding.EncodeToString(b)

	fileExtension := strings.Replace(mediaType, "image/", "", 1)
	fileUrlPath := fmt.Sprintf("http://localhost:8091/assets/%v.%v", randomPath, fileExtension)

	filePath := filepath.Join(cfg.assetsRoot, randomPath) + "." + fileExtension

	newFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file", err)
		return
	}
	_, err = io.Copy(newFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't write to file", err)
		return
	}

	videoMeta.ThumbnailURL = &fileUrlPath

	err = cfg.db.UpdateVideo(videoMeta)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, Video{
		ID:           videoMeta.ID,
		CreatedAt:    videoMeta.CreatedAt,
		UpdatedAt:    videoMeta.UpdatedAt,
		VideoURL:     videoMeta.VideoURL,
		ThumbnailURL: &fileUrlPath,
	})
}
