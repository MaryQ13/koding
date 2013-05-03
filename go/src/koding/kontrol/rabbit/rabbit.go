package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	// "github.com/koding/rabbitapi"
	"github.com/streadway/amqp"
	"io/ioutil"
	"koding/tools/config"
	"log"
	"net/http"
	"os/user"
	"strings"
)

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
}

type Producer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

type IncomingMq struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
	Vhost    string `json:"vhost"`
}

var producer *Producer

func main() {
	log.Println("kontrol rabbitproxy started")

	cred, err := authUser()
	if err != nil {
		log.Fatalln("not authorized to run!", err)
	}

	log.Println(cred)

	producer, err = createProducer()
	if err != nil {
		log.Println(err)
	}
	startRouting()
}

func authUser() (IncomingMq, error) {
	resp, err := http.DefaultClient.Get("http://localhost:3000/-/kite/login?key=123123&name=proxy")
	if err != nil {
		return IncomingMq{}, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return IncomingMq{}, err
	}

	msg := IncomingMq{}
	err = json.Unmarshal(data, &msg)
	if err != nil {
		return IncomingMq{}, err
	}

	return msg, nil
}

func startRouting() {
	c := &Consumer{
		conn:    nil,
		channel: nil,
		tag:     "",
	}

	var err error

	log.Printf("creating consumer connections")

	user := config.Current.Kontrold.RabbitMq.Login
	password := config.Current.Kontrold.RabbitMq.Password
	host := config.Current.Kontrold.RabbitMq.Host
	port := config.Current.Kontrold.RabbitMq.Port

	url := "amqp://" + user + ":" + password + "@" + host + ":" + port
	c.conn, err = amqp.Dial(url)
	if err != nil {
		log.Fatal(err)
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		log.Fatal(err)
	}

	err = c.channel.ExchangeDeclare("kontrol-rabbitproxy", "direct", false, true, false, false, nil)
	if err != nil {
		log.Fatal("exchange.declare: %s", err)
	}
	clientKey := readKey()
	if _, err := c.channel.QueueDeclare("", false, true, false, false, nil); err != nil {
		log.Fatal("queue.declare: %s", err)
	}

	log.Println("KEY is", clientKey)
	if err := c.channel.QueueBind("", clientKey, "kontrol-rabbitproxy", false, nil); err != nil {
		log.Fatal("queue.bind: %s", err)
	}

	httpStream, err := c.channel.Consume("", "", true, false, false, false, nil)
	if err != nil {
		log.Fatal("basic.consume: %s", err)
	}

	log.Println("routing started...")
	for msg := range httpStream {
		// log.Printf("got %dB message data: [%v]-[%s] %s",
		// 	len(msg.Body),
		// 	msg.DeliveryTag,
		// 	msg.RoutingKey,
		// 	msg.Body)

		body, err := doRequest(msg.Body)
		if err != nil {
			log.Println(err)
			go publishToRemote(nil, msg.CorrelationId, msg.ReplyTo)
		} else {
			go publishToRemote(body, msg.CorrelationId, msg.ReplyTo)
		}

	}
}

func doRequest(msg []byte) ([]byte, error) {
	buf := bytes.NewBuffer(msg)
	reader := bufio.NewReader(buf)
	req, err := http.ReadRequest(reader)
	if err != nil {
		log.Println(err)
	}

	// Request.RequestURI can't be set in client requests.
	// http://golang.org/src/pkg/net/http/client.go
	req.RequestURI = ""
	log.Println("Doing a http request to", req.URL.Host)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	output := new(bytes.Buffer)
	resp.Write(output)

	// log.Println("Response is", string(data))

	return output.Bytes(), nil
}

func publishToRemote(data []byte, id, routingKey string) {
	msg := amqp.Publishing{
		ContentType:   "text/plain",
		Body:          data,
		CorrelationId: id,
	}

	log.Println("publishing repsonse to", routingKey)
	err := producer.channel.Publish("kontrol-rabbitproxy", routingKey, false, false, msg)
	if err != nil {
		log.Printf("error while publishing proxy message: %s", err)
	}

}

func createProducer() (*Producer, error) {
	p := &Producer{
		conn:    nil,
		channel: nil,
	}

	log.Printf("creating publisher connections")
	var err error

	user := config.Current.Kontrold.RabbitMq.Login
	password := config.Current.Kontrold.RabbitMq.Password
	host := config.Current.Kontrold.RabbitMq.Host
	port := config.Current.Kontrold.RabbitMq.Port

	url := "amqp://" + user + ":" + password + "@" + host + ":" + port
	p.conn, err = amqp.Dial(url)
	if err != nil {
		log.Fatal(err)
	}

	p.channel, err = p.conn.Channel()
	if err != nil {
		log.Fatal(err)
	}

	return p, nil
}

func readKey() string {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	keyfile := usr.HomeDir + "/.kd/koding.key"
	log.Println(keyfile)

	file, err := ioutil.ReadFile(keyfile)
	if err != nil {
		log.Println(err)
	}

	return strings.TrimSpace(string(file))
}
