package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
)

type VideoRatio struct {
	Streams []struct {
		Width  int `json:"width,omitempty"`
		Height int `json:"height,omitempty"`
	} `json:"streams"`
}

func getVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	var buff bytes.Buffer
	cmd.Stdout = &buff
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	var videoRatio VideoRatio
	err = json.Unmarshal(buff.Bytes(), &videoRatio)
	if err != nil {
		return "", err
	}
	ratio := float64(videoRatio.Streams[0].Width) / float64(videoRatio.Streams[0].Height)

	if math.Abs(ratio-(16.0/9.0)) < 0.03 {
		return "16:9", nil
	}
	if math.Abs(ratio-(9.0/16.0)) < 0.03 {
		return "9:16", nil
	}
	return "other", nil
}

func processVideoForFastStart(filePath string) (string, error) {
	outputPath := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath)
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	return outputPath, nil
}
