package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"time"
)

const Realm int = 1
const Host string = "http://us.battle.net"
const PollInterval = 10 * time.Minute

type Match struct {
	Map      string
	Type     string
	Decision string
	Date     int64
}

type Profile struct {
	Id   int
	Name string
}

type Mailer struct {
	SmtpServer   string
	AuthUsername string
	AuthPassword string
	AuthHost     string
	AddressFrom  string
	AddressTo    string
}

type Configuration struct {
	Users  []Profile
	Groups []string
	Mailer Mailer
}

var lastPlayed time.Time
var config *Configuration

func (m Match) lastPlayed() time.Time {
	return time.Unix(m.Date, 0)
}

func main() {
	loadConfig()

	finished := make(chan bool, len(config.Users))

	// poll for each configured user
	for _, user := range config.Users {
		go poll(user.Id, user.Name, finished)
	}

	// wait for polling routines to finish
	for i := 0; i < len(config.Users); i++ {
		<-finished
	}

}

func loadConfig() {
	file, _ := os.Open("config.json")
	decoder := json.NewDecoder(file)
	config = &Configuration{}
	decoder.Decode(&config)
}

func poll(userId int, userName string, finished chan bool) {
	for {
		json := getMatchHistory(userId, userName)
		matches := parseJSON(json)
		if matches[0].lastPlayed() != lastPlayed {
			lastPlayed = matches[0].lastPlayed()
			sendMail(fmt.Sprintln(userName, "was last seen playing at", lastPlayed.Format("Jan 2, 2006 3:04pm")))
			fmt.Println(userName, "last played:", lastPlayed.Format("Jan 2, 2006 3:04pm"), matches[0])
		} else {
			fmt.Println("No new matches detected for", userName)
		}
		time.Sleep(PollInterval)
	}
	finished <- true // only used if we implement finite poll count
}

func getMatchHistory(id int, name string) []byte {
	// Example: "http://us.battle.net/api/sc2/profile/693604/1/IdrA/matches"
	url := fmt.Sprintf("%v/api/sc2/profile/%v/%v/%v/matches", Host, id, Realm, name)
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	return body
}

func parseJSON(body []byte) []Match {
	var container = make(map[string][]Match)
	err := json.Unmarshal(body, &container)
	if err != nil {
		log.Fatal(err)
	}
	return container["matches"]
}

func sendMail(message string) {
	// Set up authentication information
	auth := smtp.PlainAuth(
		"",
		config.Mailer.AuthUsername,
		config.Mailer.AuthPassword,
		config.Mailer.AuthHost,
	)

	// Body includes the To and Subject headers before the message
	body := "To: " + config.Mailer.AddressTo + "\r\nSubject: " + "Startcraft Alert" + "\r\n\r\n" + message

	// Connect to the server, authenticate, set the sender and recipient, and send the email
	err := smtp.SendMail(
		config.Mailer.SmtpServer,
		auth,
		config.Mailer.AddressFrom,
		[]string{config.Mailer.AddressTo},
		[]byte(body),
	)
	if err != nil {
		log.Fatal(err)
	}
}
