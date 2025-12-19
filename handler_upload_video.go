package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w,r.Body, 1 << 30) 
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

	
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return 

	}
	defer file.Close()

	conType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(conType)
	if err != nil{
		respondWithError(w,  http.StatusBadRequest ,"Couldn't parse Media tpe", err)
		return 
	}
	if (mediaType != "video/mp4"){
		respondWithError(w,  http.StatusBadRequest ,"Not correct media type", err)
		return
	}

	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return 
	}

	if videoData.CreateVideoParams.UserID != userID{
		respondWithError(w, http.StatusUnauthorized, "Wrong UserID ", err)
		return
	}

	extension := strings.Split(mediaType, "/")
	
	key := make([]byte, 32)
	rand.Read(key)
	encode := base64.RawURLEncoding.EncodeToString(key)
	joinedFilename := encode + "." +  extension[1]
	
	
	newFile,err := os.CreateTemp("", "tubely-upload-*.mp4")
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Cant create file", err)
		return 
	}
	defer os.Remove(newFile.Name())
	defer newFile.Close()

	_, err2 := io.Copy(newFile, file )
	if err2 != nil{
		respondWithError(w, http.StatusInternalServerError, "Cant copy file", err2)
		return 
	}

	_, err2 = newFile.Seek(0,io.SeekStart)
	cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: &cfg.s3Bucket,
		Key: 	&joinedFilename,
		Body:	newFile, 
		ContentType: &mediaType,
	})
	if err2 != nil {
		respondWithError(w, http.StatusInternalServerError, "Cant put object into s3", err2)
		return 
	}
	

	fullPath:= fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",cfg.s3Bucket, cfg.s3Region,joinedFilename )
	videoData.VideoURL = &fullPath

	err = cfg.db.UpdateVideo(videoData)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return 
	}

	respondWithJSON(w, http.StatusOK, videoData)
}
