package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

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

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return 

	}
	defer file.Close()
	mediaType := header.Header.Get("Content-Type")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type", err)
		return 

	}
	imageData,err  := io.ReadAll(file)
	if err != nil{
		respondWithError(w, http.StatusBadRequest, "Couldn't read", err)
		return 
	}
	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return 
	}

	encodedStr := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%v;base64,%v", mediaType, encodedStr)
	videoData.ThumbnailURL = &dataURL

	cfg.db.UpdateVideo(videoData)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return 
	}

	respondWithJSON(w, http.StatusOK, videoData)
}
