package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/edgexfoundry/go-mod-core-contracts/v2/dtos"
	"gocv.io/x/gocv"
	"image"
	"image/color"
	_ "image/jpeg"
	"os"

	"github.com/edgexfoundry/app-functions-sdk-go/v2/pkg"
	"github.com/edgexfoundry/app-functions-sdk-go/v2/pkg/interfaces"
	"github.com/edgexfoundry/app-functions-sdk-go/v2/pkg/transforms"
)

const (
	serviceKey = "app-face-detect"
)

func cvtImageToMat(img image.Image, matType gocv.MatType) (gocv.Mat, error) {
	bounds := img.Bounds()
	x := bounds.Dx()
	y := bounds.Dy()
	bytes := make([]byte, 0, x*y*3)

	for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
		for i := bounds.Min.X; i < bounds.Max.X; i++ {
			r, g, b, _ := img.At(i, j).RGBA()
			bytes = append(bytes, byte(b>>8), byte(g>>8), byte(r>>8))
		}
	}
	return gocv.NewMatFromBytes(y, x, matType, bytes)
}

func DetectFace(ctx interfaces.AppFunctionContext, data interface{}) (continuePipeline bool, result interface{}) {
	if data == nil {
		return false, errors.New("processImages: No data received")
	}

	event, ok := data.(dtos.Event)
	if !ok {
		return false, errors.New("processImages: didn't receive expect Event type")
	}

	// color for the rect when faces detected
	blue := color.RGBA{0, 0, 255, 0}

	// load classifier to recognize faces
	classifier := gocv.NewCascadeClassifier()
	defer classifier.Close()

	if !classifier.Load("model/haarcascade_frontalface_default.xml") {
		return false, fmt.Errorf("reading cascade file: model/haarcascade_frontalface_default.xml")
	}

	for _, reading := range event.Readings {
		// For this to work the image/jpeg & image/png packages must be imported to register their decoder
		config, _, err := image.DecodeConfig(bytes.NewReader(reading.BinaryValue))
		if err != nil {
			return false, fmt.Errorf("Failed to decode image config, error %s\n", err.Error())
		}

		var colorModel gocv.MatType
		switch config.ColorModel {
		case color.RGBAModel:
			colorModel = gocv.MatTypeCV32FC3
		case color.RGBA64Model:
			colorModel = gocv.MatTypeCV64FC3
		case color.NRGBAModel:
			colorModel = gocv.MatTypeCV32FC3
		case color.NRGBA64Model:
			colorModel = gocv.MatTypeCV64FC3
		case color.AlphaModel:
			colorModel = gocv.MatTypeCV8UC3
		case color.Alpha16Model:
			colorModel = gocv.MatTypeCV16SC3
		case color.GrayModel:
			colorModel = gocv.MatTypeCV8UC3
		case color.Gray16Model:
			colorModel = gocv.MatTypeCV16SC3
		default:
			colorModel = gocv.MatTypeCV8UC3
		}

		imageData, imageType, err := image.Decode(bytes.NewReader(reading.BinaryValue))
		if err != nil {
			return false, errors.New("processImages: unable to decode image: " + err.Error())
		}

		// Since this is a example, we will just print put some stats from the images received
		ctx.LoggingClient().Infof("Received Image from Device: %s, ResourceName: %s, Image Type: %s, Image Size: %s, Color in middle: %v\n",
			reading.DeviceName, reading.ResourceName, imageType, imageData.Bounds().Size().String(),
			imageData.At(imageData.Bounds().Size().X/2, imageData.Bounds().Size().Y/2))

		imgMat, err := cvtImageToMat(imageData, colorModel)
		if err != nil {
			return false, fmt.Errorf("Error transfer Image to gocv Mat , error %s\n", err.Error())
		}

		rects := classifier.DetectMultiScale(imgMat)
		ctx.LoggingClient().Infof("found %d faces\n", len(rects))

		// draw a rectangle around each face on the original image
		for _, r := range rects {
			gocv.Rectangle(&imgMat, r, blue, 3)
		}

		return true, imgMat
	}

	return false, nil
}

func main() {
	// turn off secure mode for examples. Not recommended for production
	_ = os.Setenv("EDGEX_SECURITY_SECRET_STORE", "false")

	// 1) First thing to do is to create an new instance of an EdgeX Application Service.
	service, ok := pkg.NewAppService(serviceKey)
	if !ok {
		os.Exit(-1)
	}

	// Leverage the built in logging service in EdgeX
	lc := service.LoggingClient()

	// 2) shows how to access the application's specific configuration settings.
	deviceNames, err := service.GetAppSettingStrings("DeviceNames")
	if err != nil {
		lc.Error(err.Error())
		os.Exit(-1)
	}

	lc.Info(fmt.Sprintf("Filtering for devices %v", deviceNames))

	// 3) This is our pipeline configuration, the collection of functions to
	// execute every time an event is triggered.
	if err := service.SetFunctionsPipeline(
		transforms.NewFilterFor(deviceNames).FilterByDeviceName,
		DetectFace); err != nil {
		lc.Errorf("SetFunctionsPipeline returned error: %s", err.Error())
		os.Exit(-1)
	}

	// 4) Lastly, we'll go ahead and tell the SDK to "start" and begin listening for events
	// to trigger the pipeline.
	err = service.MakeItRun()
	if err != nil {
		lc.Errorf("MakeItRun returned error: %s", err.Error())
		os.Exit(-1)
	}

	// Do any required cleanup here
	os.Exit(0)
}
