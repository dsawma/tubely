package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
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

	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return 
	}

	if videoData.UserID != userID{
		respondWithError(w, http.StatusUnauthorized, "Wrong UserID ", err)
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

	newFile,err := os.CreateTemp("", "tubely-upload.mp4")
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
	if err2 != nil {
		respondWithError(w, http.StatusInternalServerError, "Cant put object into s3", err2)
		return 
	}

	aspect, err  := getVideoAspectRatio(newFile.Name())
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't get aspect ratio", err)
		return 
	}
	var directory string 
	switch aspect {
	case "16:9":
		directory = "landscape"
	case "9:16":
		directory = "portrait" 
	default: 
		directory = "other" 
	}

	extension := strings.Split(mediaType, "/")
	
	key := make([]byte, 32)
	rand.Read(key)
	encode := base64.RawURLEncoding.EncodeToString(key)
	joinedFilename := encode + "." +  extension[1]

	joinedFile := filepath.Join(directory, joinedFilename)
	
	processed, err := processVideoForFastStart(newFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Cannot process file", err)
		return
	}

	processedFile,err := os.Open(processed)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Cannot process file", err)
		return
	}
	defer processedFile.Close()


	_,err=cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket: aws.String(cfg.s3Bucket),
		Key: 	aws.String(joinedFile),
		Body:	processedFile, 
		ContentType: aws.String(mediaType),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error uploading file to S3", err)
		return
	}
	fullPath:= fmt.Sprintf("https://%s/%s",cfg.s3CfDistribution, joinedFile)
	videoData.VideoURL = &fullPath
	
	err = cfg.db.UpdateVideo(videoData)
	if err != nil{
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return 
	}



	respondWithJSON(w, http.StatusOK, videoData)
}


func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err:= cmd.Run(); err != nil{
		return "", err
	}
	
	var aspect struct {
		Streams []struct {
        	Width  int `json:"width"`
        	Height int `json:"height"`
    	} `json:"streams"`
	}

	err:= json.Unmarshal(out.Bytes(), &aspect)
	if err != nil{
		return "", err
	}
	if len(aspect.Streams) == 0 {
    	return "", fmt.Errorf("no streams found")
	}

	ratio := float64(aspect.Streams[0].Width) /float64(aspect.Streams[0].Height) 
	if math.Abs(ratio-16.0/9.0) < 0.01{
		return "16:9", nil
	} else if math.Abs(ratio-9.0/16.0) < 0.01{
		return "9:16", nil 
	} else {
		return "other", nil
	}

}

func processVideoForFastStart (filePath string) (string,error){
	outputPath := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err:= cmd.Run(); err != nil{
		return "", err
	}
	return outputPath, nil
}

