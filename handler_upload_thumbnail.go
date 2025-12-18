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

	conType := header.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(conType)
	if err != nil{
		respondWithError(w,  http.StatusBadRequest ,"Couldn't parse Media tpe", err)
		return 
	}
	if (mediaType != "image/jpeg" && mediaType != "image/png"){
		respondWithError(w,  http.StatusBadRequest ,"Not correct media type", err)
		return
	}
	
	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return 
	}
	extension := strings.Split(mediaType, "/")
	
	
	
	key := make([]byte, 32)
	rand.Read(key)
	encode := base64.RawURLEncoding.EncodeToString(key)
	joinedFilename := encode + "." +  extension[1]
	joinedFile := filepath.Join(cfg.assetsRoot, joinedFilename)
	
	newFile,err := os.Create(joinedFile)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Cant create file", err)
		return 
	}
	defer newFile.Close()

	_, err2 := io.Copy(newFile, file )
	if err2 != nil{
		respondWithError(w, http.StatusInternalServerError, "Cant copy file", err2)
		return 
	}
	fullPath:= fmt.Sprintf("http://localhost:%s/assets/%s",cfg.port, joinedFilename)
	videoData.ThumbnailURL = &fullPath

	err = cfg.db.UpdateVideo(videoData)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return 
	}

	respondWithJSON(w, http.StatusOK, videoData)
}
