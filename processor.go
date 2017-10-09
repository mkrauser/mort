package mort

import (
	"errors"
	"strings"
	"io/ioutil"
	"bytes"

	Logger "github.com/labstack/gommon/log"
	"mort/config"
	"mort/engine"
	"mort/object"
	"mort/response"
	"mort/storage"
	"mort/transforms"
	"github.com/labstack/echo"
)
const S3_LOCATION_STR = "<?xml version=\"1.0\" encoding=\"UTF-8\"?><LocationConstraint xmlns=\"http://s3.amazonaws.com/doc/2006-03-01/\">EU</LocationConstraint>"

func Process(ctx echo.Context, obj *object.FileObject) *response.Response {
	switch ctx.Request().Method {
		case "GET":
			return hanldeGET(ctx, obj)
		case "PUT":
			return handlePUT(ctx, obj)

	default:
		return response.NewError(405, errors.New("method not allowed"))
	}
}

func handlePUT(ctx echo.Context, obj *object.FileObject) *response.Response {
	return storage.Set(obj, ctx.Request().Header, ctx.Request().ContentLength, ctx.Request().Body)
}

func hanldeGET(ctx echo.Context, obj *object.FileObject) *response.Response {
	if obj.Key == "" {
		return handleS3Get(ctx, obj);
	}

	var currObj *object.FileObject = obj
	var parentObj *object.FileObject = nil
	var transforms []transforms.Transforms
	var res        *response.Response
	var parentRes  *response.Response

	// search for last parent
	for currObj.HasParent() {
		if currObj.HasTransform() {
			transforms = append(transforms, currObj.Transforms)
		}
		currObj = currObj.Parent

		if !currObj.HasParent() {
			parentObj = currObj
		}
	}

	// get parent from storage
	if parentObj != nil {
		parentRes = updateHeaders(storage.Get(parentObj))

		if parentRes.StatusCode != 200 {
			return parentRes
		}
	}

	// check if object is on storage
	res = updateHeaders(storage.Get(obj))
	if res.StatusCode != 404 {
		return res
	}

	defer res.Close()

	if obj.HasTransform() && strings.Contains(parentRes.ContentType, "image/") {
		// revers order of transforms
		for i := 0; i < len(transforms)/2; i++ {
			j := len(transforms) - i - 1
			transforms[i], transforms[j] = transforms[j], transforms[i]
		}

		Logger.Infof("Performing transforms obj.Key = %s transformsLen = %s", obj.Key, len(transforms))
		return updateHeaders(processImage(obj, parentRes, transforms))
	}

	return updateHeaders(storage.Get(obj))
}

func handleS3Get(ctx echo.Context, obj *object.FileObject) *response.Response {
	req := ctx.Request()
	query := req.URL.Query()

	if _, ok := query["location"]; ok {
		return response.NewBuf(200, []byte(S3_LOCATION_STR))
	}

	return response.NewBuf(405, []byte(""))
}

func processImage(obj *object.FileObject, parent *response.Response, transforms []transforms.Transforms) *response.Response {
	engine := engine.NewImageEngine(parent)
	res, err := engine.Process(obj, transforms)
	if err != nil {
		return response.NewError(400, err)
	}

	body, _ := res.CopyBody()
	go func(buf []byte) {
		storage.Set(obj, res.Headers, res.ContentLength, ioutil.NopCloser(bytes.NewReader(buf)))

	}(body)
	return res

}

func updateHeaders(res *response.Response) *response.Response {
	headers := config.GetInstance().Headers
	for _, headerPred := range headers {
		for _, status := range headerPred.StatusCodes {
			if status == res.StatusCode {
				for h, v := range headerPred.Values {
					res.Set(h, v)
				}
				return res
			}
		}
	}
	return res
}
