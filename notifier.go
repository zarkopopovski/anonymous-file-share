package main

import (
	"gopkg.in/gomail.v2"
	"strconv"
	"strings"
)

type Notifier struct {
	config      	*Config
	receiverMail    string
	senderMail  	string
	sharedKey   	string
	sendToMe    	bool
	parameter1  	string
	parameter2  	string
}

func (notifier *Notifier) sendNotification(notificationType string) (err error) {
	config := notifier.config

	go func() {
		m := gomail.NewMessage()

		m.SetHeader("From", config.mailUsername)
		m.SetHeader("To", notifier.receiverMail)
		m.SetHeader("Subject", "DropShare: System notification")

		if notificationType == "personal" {
			m.SetBody("text/html", "Hi, you have just created new share which you can downlaod / share or delete using links bellow:<br/> share or download: <br/> http://dropshare.mediartsolutions.de/files/get-file/"+notifier.sharedKey+"<br/>or delete: <br/> http://dropshare.mediartsolutions.de/files/delete-file/"+notifier.parameter1+"<br/>or show all your drops: <br/> http://dropshare.mediartsolutions.de/my-drops/"+notifier.parameter2)

		} else if notificationType == "shared" {
			m.SetBody("text/html", "Hi, your friend "+notifier.senderMail+" has just shared one file with you which is available for download on the link bellow <br/>http://dropshare.mediartsolutions.de/files/get-file/"+notifier.sharedKey)
		} else if notificationType == "deleted" {
			var fileName = notifier.parameter1

			m.SetBody("text/html", "Hi, your file "+string(fileName[(strings.LastIndex(fileName, "$")+1):])+" has expired and is not more available for download, the file is deleted by the system.")
		} 

		port, _ := strconv.Atoi(config.mailPort)

		d := gomail.NewDialer(config.mailServer, port, config.mailUsername, config.mailPassword)

		if err := d.DialAndSend(m); err != nil {
			//panic(err)
		}
		return
	}()

	return
}
