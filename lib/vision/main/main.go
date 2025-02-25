package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"mikhailche/botcomod/lib/cloud"
	"mikhailche/botcomod/lib/vision"
	"os"
	"time"
)

func main() {
	v, err := vision.NewClient()
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	photos := []string{
		"/Users/mikhailche/Desktop/licenseplates/576ArwaKzClSlzMju19yBMqHnIo-1920.jpg",
		"/Users/mikhailche/Desktop/licenseplates/2025-02-01 16-45-12.JPG",
		"/Users/mikhailche/Desktop/licenseplates/audi.JPG",
		"/Users/mikhailche/Desktop/licenseplates/CB233_77_2.jpg",
		"/Users/mikhailche/Desktop/licenseplates/moto.JPG",
		"/Users/mikhailche/Desktop/licenseplates/moto2.JPG",
		"/Users/mikhailche/Desktop/licenseplates/note.JPG",
		"/Users/mikhailche/Desktop/licenseplates/ora.JPG",
		"/Users/mikhailche/Desktop/licenseplates/twoway.JPG",
		"/Users/mikhailche/Desktop/licenseplates/xxx.JPG",
		"/Users/mikhailche/Desktop/licenseplates/ydrive.JPG",
	}
	filepath := photos[10]
	f, _ := os.Open(filepath)
	reader := bufio.NewReader(f)
	content, err := io.ReadAll(reader)
	if err != nil {
		panic(err)
	}
	fmt.Println("Calling DetectLicensePlates")
	plates, err := v.DetectLicensePlates(ctx, "JPEG", content, cloud.WithIamToken)
	if err != nil {
		panic(err)
	}
	fmt.Println("Detected license plates:")
	for _, plate := range plates {
		fmt.Println(plate)
	}
}
