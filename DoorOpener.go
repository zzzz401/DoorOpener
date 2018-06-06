package main

import (
	"context"
	"os/signal"
	"reflect"
	"strconv"
	"time"
	//"io/ioutil"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/blackjack/webcam"
)

// CameraFormat Stores all Camera Format Information
type CameraFormat struct {
	Name        string
	FrameSize   int
	Resolutions []map[string]int
}

/*func sortCameraResolutions(Resolutions []map[string]int) []map[string]int {
	for i := range Resolutions {

	}
}*/

// Deals with V4L2 returning max and min resolutions and dups
func getCameraFormats(cam *webcam.Webcam) []CameraFormat {
	output := []CameraFormat{}

	rawFormats := cam.GetSupportedFormats()

	for frameSize, name := range rawFormats {
		rawResolutions := cam.GetSupportedFrameSizes(frameSize)
		var tempResolutions []map[string]int
		for i := 0; i < len(rawResolutions); i++ {
			tempMaxResolution := map[string]int{"width": int(rawResolutions[i].MaxWidth), "height": int(rawResolutions[i].MaxHeight)}
			tempMinResolution := map[string]int{"width": int(rawResolutions[i].MinWidth), "height": int(rawResolutions[i].MinHeight)}
			if !alreadyFoundResolution(tempResolutions, tempMaxResolution) {
				tempResolutions = append(tempResolutions, tempMaxResolution)
			}

			if !alreadyFoundResolution(tempResolutions, tempMinResolution) {
				tempResolutions = append(tempResolutions, tempMinResolution)
			}
		}
		output = append(output, CameraFormat{name, int(frameSize), tempResolutions})
	}

	return output
}

func convertResolutionSliceToString(Resolutions []map[string]int) string {
	resolutionStr := ""
	for i := range Resolutions {
		resolutionStr = resolutionStr + strconv.Itoa(Resolutions[i]["width"]) + "x" + strconv.Itoa(Resolutions[i]["height"]) + " "
	}
	return resolutionStr
}

func infoHandler() http.Handler {
	//templateChannel := make(chan template.Template)

	fn := func(writer http.ResponseWriter, request *http.Request) {
		cam, err := webcam.Open("/dev/video0")

		formats := getCameraFormats(cam)

		if err != nil {
			cameraError := "Camera Error: " + err.Error()
			fmt.Println(cameraError)
			http.Error(writer, cameraError, http.StatusInternalServerError)
			return
		}

		defer cam.Close()

		type outputFormat struct {
			Name        string
			FrameSize   int
			Resolutions string
		}

		outputFormats := []outputFormat{}

		for i := range formats {
			resolutionStr := convertResolutionSliceToString(formats[i].Resolutions)
			outputFormats = append(outputFormats, outputFormat{formats[i].Name, formats[i].FrameSize, resolutionStr})
		}

		tpl := `<!DOCTYPE html>
			<html>
				<style type="text/css">
				body, table {
					font-family:arial,sans-serif;
					height: 100%;
				}
				th {

				}
				h3 {
					margin-bottom: 0px;
				}
				table {
					border-collapse:collapse;
					border:solid 1px #999;
				}
				tr.head td {
					background-color:#FFF;
				}
				td {
					padding:5px;
					background-color:#F2F2F2;
				}
				td {
					border:solid 1px #999;
				}
				</style>
				<head>
					<meta charset="UTF-8">
					<title>Door Opener Webcam Debug</title>
				</head>
					<body>
						<h2>Door Opener Webcam Debug</h2>
						<table border="1">
							<tr>
								<tr><th colspan="3">Pixel Formats</th></tr>
								<th>Name</th><th>Frame Size</th><th>Supported Resolutions</th>
								{{range .}}
									<tr>
										<td>{{.Name}}</td>
										<td>{{.FrameSize}}</td>
										<td>{{.Resolutions}}</td>
									</tr>
								{{end}}
							</tr>
						</table>
				</body>
			</html>`

		tmpl, err := template.New("Debug").Parse(tpl)

		if err != nil {
			cameraError := "Template Error: " + err.Error()
			fmt.Println(cameraError)
			http.Error(writer, cameraError, http.StatusInternalServerError)
			return
		}

		tmpl.Execute(writer, outputFormats)
	}
	return http.HandlerFunc(fn)
}

func videoHandler(videoChannel chan []byte) http.Handler {

	fn := func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Add("Content-Type", "multipart/x-mixed-replace;boundary=MJPEGBOUNDARY")
		fmt.Println("New Connection From: " + request.RemoteAddr)
		for {
			writer.Write(<-videoChannel)
		}
	}
	return http.HandlerFunc(fn)
}

/*func videoHandler(writer http.ResponseWriter, request *http.Request) {
	videoChannel := make(chan []byte)
	writer.Header().Add("Content-Type", "multipart/x-mixed-replace;boundary=MJPEGBOUNDARY")
	go readVideoStream(videoChannel)
	for {
		writer.Write(<-videoChannel)
	}
}*/

func readVideoStream(videoChannel chan []byte) {
	cam, err := webcam.Open("/dev/video0")

	if err != nil {
		cameraError := "Camera Error: " + err.Error()
		fmt.Println(cameraError)
		//http.Error(writer, cameraError, http.StatusInternalServerError)
		return
	}

	defer cam.Close()

	formats := cam.GetSupportedFormats()

	var MJPEGPixelFormat webcam.PixelFormat

	for pixelFormat, formatName := range formats {
		if formatName == "Motion-JPEG" {
			MJPEGPixelFormat = pixelFormat
		}
	}

	var videoWidth uint32 = 1280
	var videoHeight uint32 = 720

	_, _, _, err = cam.SetImageFormat(MJPEGPixelFormat, videoWidth, videoHeight)

	if err != nil {
		cameraError := "Camera Error: " + err.Error()
		fmt.Fprint(os.Stderr, cameraError)
		//http.Error(writer, cameraError, http.StatusInternalServerError)
		return
	}

	err = cam.StartStreaming()

	if err != nil {
		cameraError := "Camera Error: " + err.Error()
		fmt.Fprint(os.Stderr, cameraError)
		//http.Error(writer, cameraError, http.StatusInternalServerError)
		return
	}

	//writer.Header().Add("Content-Type", "multipart/x-mixed-replace;boundary=MJPEGBOUNDARY")

	timeout := uint32(1) //5 seconds
	/*frameTicker := time.NewTicker(time.Second)
	var frameCount int*/

	for {
		/*go func() {
			for {
				select {
				case <-frameTicker.C:
					fmt.Println(strconv.Itoa(frameCount) + " fps")
					frameCount = 0
				}
			}
		}()*/

		err = cam.WaitForFrame(timeout)

		switch err.(type) {
		case nil:
		case *webcam.Timeout:
			fmt.Fprint(os.Stderr, "Camera Error: "+err.Error())
			continue
		default:
			cameraError := "Camera Error: " + err.Error()
			fmt.Fprint(os.Stderr, cameraError)
			//http.Error(writer, cameraError, http.StatusInternalServerError)
			return
		}

		rawFrame, err := cam.ReadFrame()
		if len(rawFrame) != 0 {
			header := "\r\n--MJPEGBOUNDARY\r\nContent-Type: image/jpeg\r\nContent-Length: " + fmt.Sprint(len(rawFrame)) + "\r\nX-Timestamp: 0.000000\r\n\r\n"
			frame := append([]byte(header), []byte(rawFrame)...)
			//writer.Write(frame)
			videoChannel <- frame
			//frameCount++
		} else if err != nil {
			cameraError := "Camera Error: " + err.Error()
			fmt.Fprint(os.Stderr, cameraError)
			//http.Error(writer, cameraError, http.StatusInternalServerError)
			return
		}
	}
}

func alreadyFoundResolution(resolutions []map[string]int, newResolution map[string]int) bool {
	for i := range resolutions {
		if reflect.DeepEqual(resolutions[i], newResolution) {
			return true
		}
	}
	return false
}

type customLogWriter struct{}

func (writer customLogWriter) Write(bytes []byte) (int, error) {
	return fmt.Print(time.Now().Format("2006-01-02 15:04:05") + ": " + string(bytes))
}

func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func shutdown(server *http.Server, logger *log.Logger, quit <-chan os.Signal, done chan<- bool) {
	// Blocking Channel
	<-quit
	logger.Println("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server.SetKeepAlivesEnabled(false)
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
		os.Exit(1)
	}

	close(done)
}

func basicHandler(router *http.ServeMux, logger *log.Logger) http.Handler {
	handlerFn := func(writer http.ResponseWriter, request *http.Request) {
		requestID := request.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = generateRequestID()
		}

		//Compiler dosen't like passing primitves into context.WithValue
		type key int
		var requestIDKey key

		ctx := context.WithValue(request.Context(), requestIDKey, requestID)
		writer.Header().Set("X-Request-Id", requestID)

		defer func() {
			logger.Println(requestID, request.Method, request.URL.Path, request.RemoteAddr, request.UserAgent())
		}()

		router.ServeHTTP(writer, request.WithContext(ctx))
	}

	return http.HandlerFunc(handlerFn)
}

func main() {
	listenAddr := ":8080"

	logger := log.New(new(customLogWriter), "", 0)

	logger.Println("Server is starting...")

	router := http.NewServeMux()
	router.Handle("/info", infoHandler())

	videoChannel := make(chan []byte)
	go readVideoStream(videoChannel)

	router.Handle("/video.mjpg", videoHandler(videoChannel))

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	server := &http.Server{
		Addr:     listenAddr,
		ErrorLog: logger,
		Handler:  basicHandler(router, logger),
		//ReadTimeout:  5 * time.Second,
		//WriteTimeout: 10 * time.Second,
		//	IdleTimeout:    15 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go shutdown(server, logger, quit, done)

	logger.Println("Web Server Listening on ", listenAddr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}

	// Blocking Channel
	<-done
	logger.Println("Server stopped")
	os.Exit(0)
}
