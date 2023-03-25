package main

import (
	"database/sql"
	"fmt"
	"strconv"

	_ "github.com/mattn/go-sqlite3"

	"crypto/sha1"
	"time"
	"strings"

	"github.com/teris-io/shortid"
)

type DBManager struct {
	db *sql.DB
}

type FileModel struct {
	Id            string   `json:"id"`
	Parameter     string   `json:"parameter"`
	Parameter2     string  `json:"parameter2"`
	FileName      string   `json:"file_name"`
	FileSize      int64    `json:"file_size"`
	Timestamp     string   `json:"timestamp"`
	ReceiverEmail string   `json:"receiver_email"`
	InPath		  string   `json:"in_path"`
}

func CreateDBConnection() *DBManager {
	dbManager := &DBManager{}
	dbManager.OpenConnection()

	return dbManager
}

func (dbConnection *DBManager) OpenConnection() (err error) {
	db, err := sql.Open("sqlite3", "./afserv.db")
	if err != nil {
		panic(err)
	}

	fmt.Println("SQLite Connection is Active")
	dbConnection.db = db

	dbConnection.setupInitialDatabase()

	return
}

func (dbConnection *DBManager) setupInitialDatabase() (err error) {
	statement, _ := dbConnection.db.Prepare("CREATE TABLE IF NOT EXISTS files (id VARCHAR PRIMARY KEY, share_key VARCHAR, delete_key VARCHAR, file_name VARCHAR, file_size INTEGER, timestamp VARCHAR, in_path VARCHAR, receiver_email VARCHAR, date_created VARCHAR, date_modified VARCHAR)")
	statement.Exec()

	statement, _ = dbConnection.db.Prepare("CREATE TABLE IF NOT EXISTS stats (id INTEGER PRIMARY KEY AUTOINCREMENT, operation VARCHAR, file_size INTEGER, date_created VARCHAR)")
	statement.Exec()

	return
}

func (dbConnection *DBManager) addNewFile(parameter1 string, fileName string, fileSize int64, lifeTime string, inPath string) (shareKey string, deleteID string, err error) {
	sha1Hash := sha1.New()
	sha1Hash.Write([]byte(time.Now().String() + parameter1 + fileName))
	sha1HashString := sha1Hash.Sum(nil)

	fileID := fmt.Sprintf("%x", sha1HashString)

	sha1Hash.Write([]byte(time.Now().String() + "d3LEt3F1l3" + parameter1 + fileName))
	sha1HashString = sha1Hash.Sum(nil)

	deleteID = fmt.Sprintf("%x", sha1HashString)

	sid, err := shortid.New(1, shortid.DefaultABC, 2342)
	shortid.SetDefault(sid)

	shareKey, err = sid.Generate()

	now := time.Now()

	var afterX time.Time

	var timeAfter = string(lifeTime[0:len(lifeTime) - 1])
	value, _ := strconv.Atoi(timeAfter)
	
	if string(lifeTime[len(lifeTime) - 1]) == "m" {
		afterX = now.Add(time.Duration(value) * time.Minute)
	} else if string(lifeTime[len(lifeTime) - 1]) == "h" {
		afterX = now.Add(time.Duration(value) * time.Hour)
	} else if string(lifeTime[len(lifeTime) - 1]) == "d" {
		afterX = now.AddDate(0, 0, value)
	}

	timeInMillis := afterX.Unix()
	
	fmt.Println("New file uploaded with lifetime:", timeInMillis)

	query := "INSERT INTO files(id, share_key, delete_key, file_name, file_size, timestamp, in_path, date_created, date_modified) VALUES($1, $2, $3, $4, $5, $6, $7, datetime('now'), datetime('now'))"

	_, err = dbConnection.db.Exec(query, fileID, shareKey, deleteID, fileName, fileSize, timeInMillis, inPath)

	if err == nil {
		return shareKey, deleteID, nil
	}

	panic(err)

	return
}

func (dbConnection *DBManager) findUserFileByID(sharedKey string) *FileModel {
	query := "SELECT delete_key, file_name, file_size FROM files WHERE share_key=$1"

	userFile := new(FileModel)

	err := dbConnection.db.QueryRow(query, sharedKey).Scan(&userFile.Parameter, &userFile.FileName, &userFile.FileSize)

	if err != nil {
		return nil
	}

	return userFile
}

func (dbConnection *DBManager) findAllFilesRecord() []*FileModel {
	query := "SELECT delete_key, file_name, file_size, timestamp, receiver_email FROM files"

	rows, err := dbConnection.db.Query(query)

	if err == nil {
		usersFiles := make([]*FileModel, 0)

		for rows.Next() {

			userFile := new(FileModel)

			_ = rows.Scan(&userFile.Parameter, &userFile.FileName, &userFile.FileSize, &userFile.Timestamp, &userFile.ReceiverEmail)

			usersFiles = append(usersFiles, userFile)

		}

		return usersFiles
	}

	return nil
}

func (dbConnection *DBManager) findUserFileByDeleteID(deleteID string) *FileModel {
	query := "SELECT file_name, file_size FROM files WHERE delete_key=$1"

	userFile := new(FileModel)

	err := dbConnection.db.QueryRow(query, deleteID).Scan(&userFile.FileName, &userFile.FileSize)

	if err != nil {
		return nil
	}

	return userFile
}

func (dbConnection *DBManager) findAllUserFilesRecord(fileKey string) []*FileModel {
	query := "SELECT share_key, delete_key, file_name, file_size, timestamp, in_path, receiver_email FROM files WHERE in_path=?"

	rows, err := dbConnection.db.Query(query, fileKey)

	//param1, fileName, fileSize, receiverEmail, timeStamp, inPath := "", "", int64(0), "", "", ""
	fileName := ""

	if err == nil {
		usersFiles := make([]*FileModel, 0)

		for rows.Next() {
			userFile := new(FileModel)

			_ = rows.Scan(&userFile.Parameter, &userFile.Parameter2, &fileName, &userFile.FileSize, &userFile.Timestamp, &userFile.InPath, &userFile.ReceiverEmail)
			
			userFile.FileName = string(fileName[(strings.LastIndex(fileName, "$")+1):])
			//_ = rows.Scan(&param1, &fileName, &fileSize, &receiverEmail, &timeStamp, &inPath)
			/*
			userFile := &FileModel {
				Parameter: param1,
				FileName: fileName,
				FileSize: fileSize,
				ReceiverEmail: receiverEmail,
				Timestamp: timeStamp,
				InPath: inPath,
			}
			*/
			usersFiles = append(usersFiles, userFile)

		}
		
		return usersFiles
	}

	return nil
}

func (dbConnection *DBManager) markFileAsDeleted(fileID string) (err error) {
	query := "UPDATE files SET deleted=1, date_modified=datetime('now') WHERE id=$1"

	_, err = dbConnection.db.Exec(query, fileID)

	if err == nil {
		return nil
	}

	panic(err)

	return
}

func (dbConnection *DBManager) deleteFileData(fileID string) (err error) {
	query := "DELETE FROM files WHERE delete_key=$1"

	_, err = dbConnection.db.Exec(query, fileID)

	if err == nil {
		return nil
	}

	panic(err)

	return
}

func (dbConnection *DBManager) addStatsForAction(actionType string, fileSize int64) (err error) {
	query := "INSERT INTO stats(operation, file_size, date_created) VALUES($1, $2, datetime('now'))"

	_, err = dbConnection.db.Exec(query, actionType, fileSize)

	if err == nil {
		return nil
	}

	panic(err)

	return
}

func (dbConnection *DBManager) findWholeFileTraffic() string {
	var FileSize string
	var FileSizeConverted float64
	query := "SELECT SUM(file_size) AS file_size FROM stats"

	err := dbConnection.db.QueryRow(query).Scan(&FileSize)

	if err != nil {
		return "0"
	}

	FileSizeConverted, err = strconv.ParseFloat(FileSize, 64)

	FileSizeConverted = (FileSizeConverted / 1024) / 1024
	stringed := fmt.Sprintf("%.2f", FileSizeConverted)
	
	return stringed
}

func (dbConnection *DBManager) updateFileEmailNotification(token string, email string) (err error) {
	query := "UPDATE files SET receiver_email=$1 WHERE share_key=$2"

	_, err = dbConnection.db.Exec(query, email, token)

	if err == nil {
		return nil
	}

	panic(err)

	return
}

func (dbConnection *DBManager) updateFileLifeTime(token string, lifeTime string) (err error) {
	now := time.Now()

	var afterX time.Time

	var timeAfter = string(lifeTime[0:len(lifeTime) - 1])
	value, _ := strconv.Atoi(timeAfter)
	
	if string(lifeTime[len(lifeTime) - 1]) == "m" {
		afterX = now.Add(time.Duration(value) * time.Minute)
	} else if string(lifeTime[len(lifeTime) - 1]) == "h" {
		afterX = now.Add(time.Duration(value) * time.Hour)
	} else if string(lifeTime[len(lifeTime) - 1]) == "d" {
		afterX = now.AddDate(0, 0, value)
	}

	timeInMillis := afterX.Unix()

	query := "UPDATE files SET timestamp=$1 WHERE share_key=$2"

	_, err = dbConnection.db.Exec(query, timeInMillis, token)

	if err == nil {
		return nil
	}

	panic(err)

	return
}
