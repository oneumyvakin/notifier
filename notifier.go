package notifier

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

const (
	NotifyAlways   int = 0
	NotifyOnceHour int = 1
	NotifyOnceDay  int = 2
)

// Notifier sends email notification by SendGrid tagged by MessageTag with Frequency
type Notifier struct {
	ApiHost          string // Default is "https://api.sendgrid.com"
	ApiKey           string // SendGrid API key, required
	Log              *log.Logger
	From             Recipient
	To               []Recipient // Required
	Frequency        int
	MessageTag       string
	DatabaseFilePath string
}

// Recipient holds Title and email Address
type Recipient struct {
	Title   string
	Address string
}

// Send message with subject
func (notifier Notifier) Send(subject string, message string) error {
	if err := notifier.setDefaults(); err != nil {
		return fmt.Errorf("Notifier failed to send message: %s", err)
	}

	hash := notifier.getHash()
	if !notifier.needToSend(hash) {
		notifier.Log.Printf("Skip message %s: %s %s", hash, subject, message)
		return nil
	}

	notifier.Log.Printf("Send message %s: %s %s", notifier.MessageTag, subject, message)

	from := mail.NewEmail(notifier.From.Title, notifier.From.Address)
	for _, recipient := range notifier.To {
		to := mail.NewEmail(recipient.Title, recipient.Address)
		content := mail.NewContent("text/plain", message)
		m := mail.NewV3MailInit(from, subject, to, content)

		request := sendgrid.GetRequest(notifier.ApiKey, "/v3/mail/send", notifier.ApiHost)
		request.Method = "POST"
		request.Body = mail.GetRequestBody(m)
		response, err := sendgrid.API(request)
		if err != nil {
			notifier.Log.Printf("Notifier failed to send message: %s", err)
			return err
		}
		notifier.Log.Printf("Message sent: %s", response)
	}

	err := notifier.addToDb(hash, subject)
	if err != nil {
		notifier.Log.Printf("Notifier failed to send message: %s", err)
		return err
	}

	return nil
}

func (notifier Notifier) setDefaults() error {
	if notifier.ApiKey == "" {
		return errors.New("SendGrid API key is not set")
	}

	if len(notifier.To) == 0 {
		return errors.New("Recipients are not set")
	}

	if notifier.From.Address == "" {
		notifier.From.Address = "no-reply@no-where.tld"
		notifier.From.Title = "SendGrid Notifier"
	}

	if notifier.ApiHost == "" {
		notifier.ApiHost = "https://api.sendgrid.com"
	}

	if notifier.Log == nil {
		notifier.Log = log.New(os.Stdout, "SendGrid Notifier ", log.LstdFlags)
	}

	if notifier.MessageTag == "" {
		notifier.MessageTag = "default_tag"
	}

	if notifier.DatabaseFilePath == "" {
		notifier.DatabaseFilePath = "notifier.json"
	}

	return nil
}

func (notifier Notifier) needToSend(hash string) bool {
	if notifier.Frequency == NotifyAlways {
		return true
	}

	if notifier.inDb(hash) {
		return false
	}

	return true
}

func (notifier Notifier) getHash() (hash string) {
	t := time.Now()

	if notifier.Frequency == NotifyOnceHour {
		hash = t.Format("2006-01-02-15") + ":" + notifier.MessageTag
		return
	}

	if notifier.Frequency == NotifyOnceDay {
		hash = t.Format("2006-01-02") + ":" + notifier.MessageTag
		return
	}

	return
}

func (notifier Notifier) addToDb(hash string, subject string) error {
	db, err := notifier.loadDb()
	if err != nil {
		return fmt.Errorf("Notifier failed to add %s:%s to database: %s", hash, subject, err)
	}

	db[hash] = subject

	err = notifier.saveDb(db)
	if err != nil {
		return fmt.Errorf("Notifier failed to add %s to database: %s", hash, err)
	}

	return nil
}

func (notifier Notifier) inDb(hash string) bool {
	db, err := notifier.loadDb()
	if err != nil {
		notifier.Log.Printf("Notifier failed to load database: %s", err)
		return false
	}

	if _, ok := db[hash]; ok == true {
		return true
	}

	return false
}

func (notifier Notifier) saveDb(db map[string]string) (err error) {
	if notifier.DatabaseFilePath == "" {
		notifier.DatabaseFilePath = "notifier.json"
	}

	dbFile, err := os.Open(notifier.DatabaseFilePath)
	if err != nil {
		return fmt.Errorf("Notifier failed to save database: %s", err)
	}
	defer dbFile.Close()

	err = json.NewEncoder(dbFile).Encode(db)
	if err != nil {
		return fmt.Errorf("Notifier failed to save database: %s", err)
	}

	return
}

func (notifier Notifier) loadDb() (db map[string]string, err error) {
	if notifier.DatabaseFilePath == "" {
		notifier.DatabaseFilePath = "notifier.json"
	}

	dbFile, err := os.Open(notifier.DatabaseFilePath)
	if err != nil {
		dbFile, err = notifier.createDb()
		if err != nil {
			return nil, fmt.Errorf("Notifier failed to load database: %s", err)
		}
	}
	defer dbFile.Close()

	db = make(map[string]string)
	err = json.NewDecoder(dbFile).Decode(&db)
	if err != nil {
		return nil, fmt.Errorf("Notifier failed to load database: %s", err)
	}

	return
}

func (notifier Notifier) createDb() (dbFile *os.File, err error) {
	dbFile, err = os.Create(notifier.DatabaseFilePath)
	if err != nil {
		return nil, fmt.Errorf("Notifier failed to create database file %s: %s", notifier.DatabaseFilePath, err)
	}
	err = dbFile.Truncate(0)
	if err != nil {
		return nil, fmt.Errorf("Notifier failed to create database file %s: %s", notifier.DatabaseFilePath, err)
	}
	_, err = dbFile.Write([]byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("Notifier failed to create database file %s: %s", notifier.DatabaseFilePath, err)
	}
	return
}
