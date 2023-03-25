package main

import (
	"crypto/sha1"
	"encoding/json"

	"fmt"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"

	"io"
	//"io/ioutil"
	"math/rand"
	"os"
	"strconv"

	"strings"

	"github.com/teris-io/shortid"

	//"bytes"
	//"log"
)

type FileController struct {
	dbManager *DBManager
	config	*Config
}

const MAX_UPLOAD_SIZE = 1024 * 1024 * 50// 50MB

func (fController *FileController) uploadFile(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_SIZE)
	if errSize := r.ParseMultipartForm(MAX_UPLOAD_SIZE); errSize != nil {
		http.Error(w, "The uploaded file is too big. Please choose an file that's less than 50MB in size", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")

	lifeTime := r.FormValue("life_time")

	inPath := r.FormValue("in_path")

	parameter1 := r.FormValue("parameter_1")
	if parameter1 == "" {
		parameter1 = "S3cREtF1L3Up&0@d"
	}

	if err != nil {
		fmt.Fprintln(w, err)
		return
	}

	defer file.Close()

	fileName := header.Filename

	randomFloat := strconv.FormatFloat(rand.Float64(), 'E', -1, 64)

	sha1Hash := sha1.New()
	sha1Hash.Write([]byte(time.Now().String() + parameter1 + fileName + randomFloat))
	sha1HashString := sha1Hash.Sum(nil)

	fileNameHASH := fmt.Sprintf("%x", sha1HashString)

	fileName = fileNameHASH + "$" + fileName

	out, err := os.Create("./uploads/" + fileName)

	if err != nil {
		fmt.Fprintf(w, "Unable to create a file for writting. Check your write access privilege")
		return
	}

	defer out.Close()

	_, err = io.Copy(out, file)

	if err != nil {
		fmt.Fprintln(w, err)
	} 

	fileID, deleteID, errDB := fController.dbManager.addNewFile(parameter1, fileName, header.Size, lifeTime, inPath)

	fController.dbManager.addStatsForAction("UPLOAD", header.Size)

	if errDB == nil {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(map[string]string{"status": "success", "error_code": "-1", "file_token": fileID, "delete_token": deleteID}); err != nil {
			w.WriteHeader(http.StatusNotFound)
			panic(err)
		}
		
		return
	}
		
	http.Error(w, "Saving failed.", http.StatusBadRequest)
}

func (fController * FileController) updateLifeTime(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	sharedKey := r.FormValue("shared_key")
	lifeTime := r.FormValue("life_time")

	errData := fController.dbManager.updateFileLifeTime(sharedKey, lifeTime)

	if errData != nil {
    	w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error":"0"}); err != nil {
			panic(err)
		}
		return
    }

    w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success", "error_code": "-1"}); err != nil {
		panic(err)
	}
}

func (fController * FileController) getSpecificFile(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	sharedKey := params.ByName("fileToken")

	fileModel := fController.dbManager.findUserFileByID(sharedKey)

	if fileModel == nil {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error":"0"}); err != nil {
			panic(err)
		}

		return
	}

	fController.dbManager.addStatsForAction("DOWNLOAD", fileModel.FileSize)

	realFileName := strings.Split(fileModel.FileName, "$")[1]

	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(realFileName))
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, "./uploads/"+fileModel.FileName)
}

func (fController *FileController) deleteFile(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	fileID := params.ByName("fileToken")

	fileModel := fController.dbManager.findUserFileByDeleteID(fileID)

	err := os.Remove("./uploads/" + fileModel.FileName)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")	

    if err != nil {
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error":"0"}); err != nil {
			panic(err)
		}
		return
    }

    errData := fController.dbManager.deleteFileData(fileID)

    if errData != nil {
    	w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error":"0"}); err != nil {
			panic(err)
		}
		return
    }

    w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success", "error_code": "-1"}); err != nil {
		panic(err)
	}
}

func (fController *FileController) sendSharedLinkOnEmail(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	sender := r.FormValue("sender_email")
	receiver := r.FormValue("receiver_email")
	sharedKey := r.FormValue("shared_key")
	shareType := r.FormValue("share_type")

	notifyMe := r.FormValue("notify_me")
	dropsPath := r.FormValue("drops_path")

	if notifyMe == "yes" {
		//Update db record with receiver email for notification
		_ = fController.dbManager.updateFileEmailNotification(sharedKey, receiver)
	}

	var notifier *Notifier

	fileModel := fController.dbManager.findUserFileByID(sharedKey)

	if shareType == "personal" {
		notifier = &Notifier{
			config:   fController.config,	
			receiverMail: receiver,
			sharedKey: sharedKey,
			parameter1: fileModel.Parameter,
			parameter2: dropsPath,
		}
	} else if shareType == "shared" {
		notifier = &Notifier{
			config:   fController.config,	
			receiverMail: receiver,
			senderMail: sender,
			sharedKey: sharedKey,
		}
	}

	notifier.sendNotification(shareType)
}

func (fController *FileController) checkWholeTraffic(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	FileTraffic := fController.dbManager.findWholeFileTraffic()

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success", "error_code": "-1", "file_traffic": FileTraffic}); err != nil {
		w.WriteHeader(http.StatusNotFound)
		panic(err)
	}
}

func (fController *FileController) generateSecretPath(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	sid, _ := shortid.New(1, shortid.DefaultABC, 2342)
	shortid.SetDefault(sid)

	secretPathKey, _ := sid.Generate()

	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success", "error_code": "-1", "secret_path_key": secretPathKey}); err != nil {
		w.WriteHeader(http.StatusNotFound)
		panic(err)
	}
}

func (fController *FileController) findAllUserFiles(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	filesKey := r.FormValue("in_path")

	filesData := fController.dbManager.findAllUserFilesRecord(filesKey)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

    if filesData == nil {
    	w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error":"0"}); err != nil {
			panic(err)
		}
		return
    }

    w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	filesJSONData, _ := json.Marshal(filesData)

	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success", "error_code": "-1", "data": string(filesJSONData)}); err != nil {
		panic(err)
	}
}
