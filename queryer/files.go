package queryer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"

	"github.com/buildbuildio/pebbles/requests"
)

type UploadMapItem struct {
	upload    *requests.Upload
	positions []string
}

type UploadMap []*UploadMapItem

func (u UploadMap) Map() map[string][]string {
	var result = make(map[string][]string)

	for idx, attachment := range u {
		result[strconv.Itoa(idx)] = attachment.positions
	}

	return result
}

func (u UploadMap) Empty() bool {
	return len(u) == 0
}

func (u *UploadMap) Add(upload *requests.Upload, varName string) {
	*u = append(*u, &UploadMapItem{
		upload,
		[]string{fmt.Sprintf("variables.%s", varName)},
	})
}

// function extracts attached files and sets respective variables to null
func extractFiles(input *requests.Request) *UploadMap {
	uploadMap := &UploadMap{}
	if input == nil {
		return uploadMap
	}
	for varName, value := range input.Variables {
		uploadMap.extract(value, varName)
		// if the value was an upload, set the respective Request variable to null
		if _, ok := value.(*requests.Upload); ok {
			input.Variables[varName] = nil
		}
	}
	return uploadMap
}

func (u *UploadMap) extract(value interface{}, path string) {
	switch val := value.(type) {
	case *requests.Upload: // Upload found
		u.Add(val, path)
	case map[string]interface{}:
		for k, v := range val {
			u.extract(v, fmt.Sprintf("%s.%s", path, k))
			// if the value was an upload, set the respective QueryInput variable to null
			switch v.(type) {
			case *requests.Upload, requests.Upload:
				val[k] = nil
			}
		}
	case []interface{}:
		for i, v := range val {
			u.extract(v, fmt.Sprintf("%s.%d", path, i))
			// if the value was an upload, set the respective QueryInput variable to null
			switch v.(type) {
			case *requests.Upload, requests.Upload:
				val[i] = nil
			}
		}
	}
}

func prepareMultipart(payload []byte, uploadMap UploadMap) (body []byte, contentType string, err error) {
	var b = bytes.Buffer{}
	var fw io.Writer

	w := multipart.NewWriter(&b)

	fw, err = w.CreateFormField("operations")
	if err != nil {
		return
	}

	_, err = fw.Write(payload)
	if err != nil {
		return
	}

	fw, err = w.CreateFormField("map")
	if err != nil {
		return
	}

	err = json.NewEncoder(fw).Encode(uploadMap.Map())
	if err != nil {
		return
	}

	for index, uploadVariable := range uploadMap {
		fw, e := w.CreateFormFile(strconv.Itoa(index), uploadVariable.upload.FileName)
		if e != nil {
			return b.Bytes(), w.FormDataContentType(), e
		}

		_, e = io.Copy(fw, uploadVariable.upload.File)
		if e != nil {
			return b.Bytes(), w.FormDataContentType(), e
		}
	}

	err = w.Close()
	if err != nil {
		return
	}

	return b.Bytes(), w.FormDataContentType(), nil
}
