package main

import (
	"log"
	"net/http"
	"fmt"
	"time"
	"os"
	"os/signal"
	"context"

	"strconv"
	//"io"
	"io/ioutil"
	"github.com/julienschmidt/httprouter"
	"github.com/kenshaw/ini"

	"github.com/rs/cors"
)

type GFServCore struct {
	dbManager      *DBManager
	fController *FileController
	config	*Config
}

type Config struct {
	mailServer   string
	mailPort     string
	mailUsername string
	mailPassword string
	httpPort	   string
}

func CreateGFService(config *Config) *GFServCore {
	gfServiceCore := &GFServCore{
		dbManager:      CreateDBConnection(),
		fController: &FileController{},
		config:       config,
	}

	gfServiceCore.fController.dbManager = gfServiceCore.dbManager
	gfServiceCore.fController.config = gfServiceCore.config

	return gfServiceCore
}

func CreateNewRouter(handlers *GFServCore) *httprouter.Router {
	router := httprouter.New()

	router.GET("/", index)
	router.GET("/my-drops/:key", index)
	router.POST("/files/upload-file", handlers.fController.uploadFile)
	router.POST("/files/life-time", handlers.fController.updateLifeTime)
	router.GET("/files/get-file/:fileToken", handlers.fController.getSpecificFile)
	router.GET("/files/delete-file/:fileToken", handlers.fController.deleteFile)
	router.POST("/files/share-file", handlers.fController.sendSharedLinkOnEmail)

	router.GET("/files/generate-path", handlers.fController.generateSecretPath)
	router.POST("/files/list-path", handlers.fController.findAllUserFiles)

	router.GET("/files/get-traffic", handlers.fController.checkWholeTraffic)

	router.ServeFiles("/web/*filepath", http.Dir("./web"))

	//router.ServeFiles("/uploads/*filepath", http.Dir("./uploads"))

	return router
}

func index(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
        index, err := ioutil.ReadFile("./web/index.html")

        //panic(err)

        if err != nil {
            panic(err)
            return
        }

        //w.Header().Set("Access-Control-Allow-Origin", "*")
        //w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
        w.Header().Set("Content-Type", "text/html; charset=utf-8")

        fmt.Fprintf(w, string(index))
}

func releaseAllExpiredFiles(gfService *GFServCore) {
	//LOAD ALL EXISTING FILES RECORDS FROM DB
	availableFiles := gfService.dbManager.findAllFilesRecord()

	if availableFiles != nil && len(availableFiles) > 0 {
		//LOOP THRU EACH OF THEM AND CHECK LIFETIME FLAG + DATE CREATED
		now := time.Now()
		timeInMillis := now.Unix()

		for _, fileObj := range availableFiles {
			//TEST IF CURRENT TIME IS EQUAL TO DATE CREATED + LIFETIME FLAG
			fileLifeTime, _ := strconv.Atoi(fileObj.Timestamp)
			//fmt.Println("File Name:", fileObj.FileName, " Timestamp:",fileObj.Timestamp, " File Lifetime:", fileLifeTime, " Current Time In Millis:", timeInMillis)
			if timeInMillis >= int64(fileLifeTime) {
				//fmt.Println("DELETED File Name:", fileObj.FileName)
			
				//DELETE THE FILE IF THE LIFETIME IS OVER AND REMOVE THE RECORD FORM THE DB
				_ = os.Remove("./uploads/" + fileObj.FileName)
				_ = gfService.dbManager.deleteFileData(fileObj.Parameter)

				if fileObj.ReceiverEmail != "" {
					//SEND NOTIFICATION FOR DELETED FILE
					notifier := &Notifier{
						config:   gfService.config,	
						receiverMail: fileObj.ReceiverEmail,
						parameter1: fileObj.FileName,
					}

					notifier.sendNotification("deleted")
				}
			}
		}
	}
}

func main() {
	fileCfg, err := ini.LoadFile("config.cfg")
	if err != nil {
		log.Fatal("Error with service configuration %s", err)
	}

	port := fileCfg.GetKey("service-1.port")

	if port == "" {
		log.Fatal("Error with port number configuration")
	}

	config := &Config{
		mailServer:   fileCfg.GetKey("service-1.mailserver"),
		mailPort:     fileCfg.GetKey("service-1.mailport"),
		mailUsername: fileCfg.GetKey("service-1.mailusername"),
		mailPassword: fileCfg.GetKey("service-1.mailpassword"),
		httpPort:     port,
	}

	gfService := CreateGFService(config)
	router := CreateNewRouter(gfService)

	go func(gfService *GFServCore) {
		ticker := time.NewTicker(5 * time.Minute)
		  
		for _ = range ticker.C {
			releaseAllExpiredFiles(gfService)

			//time.Sleep(30 * time.Minute)
		}
	}(gfService)

	handler := cors.Default().Handler(router)
	
	logger := log.New(os.Stdout, "anon-file-share", log.LstdFlags)
	
	thisServer := &http.Server{
		Addr:				  ":"+port,
		Handler:	 	  handler,
		IdleTimeout:   120 * time.Second,
		ReadTimeout:	 1 * time.Second,
		WriteTimeout:	1 * time.Second,	
	}
	
	go func() {
		err := thisServer.ListenAndServe()
		if err != nil {
			logger.Fatal(err)
		}
	}()
	
	sigChan:= make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, os.Kill)
	
	thisSignalChan := <-sigChan
	
	logger.Println("Graceful Shutdown", thisSignalChan)
	
	timeOutContext, canFunct := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer canFunct()
	
	thisServer.Shutdown(timeOutContext)

	//log.Fatal(http.ListenAndServe(":"+port, handler))
}
