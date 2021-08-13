package main

import (
	"bufio"
	"expvar"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

var (
	logger          *log.Logger
	startTime       time.Time
	allUsers        []user
	rooms           = make(map[string][]user)
	roomsHistory    = make([][]string, 0, 10)
	categories      = make([]string, 0, 10)
	subChannel      = make(chan []interface{})
	newCatChannel   = make(chan [2]interface{})
	joinChan        = make(chan net.Conn)
	requestsMonitor = expvar.NewInt("Total Requests")
	invalidRequests = expvar.NewInt("Total invalid requests")
	totalUsers      = expvar.NewInt(("Total Users"))
	newMessage      = make(chan [3]string)
)

type user struct {
	name    string
	address net.Conn
}

func init() {
	file, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	logger = log.New(file, "", log.Ldate|log.Ltime|log.Lshortfile)

}

func startTCP() {
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			logger.Println("user failed to connect")
		}
		logger.Printf("User %v connected\n", conn)
		io.WriteString(conn, "Welcome to the message hub!\n Write [CMD] for a list of commands\n")
		joinChan <- conn

		go handler(conn)

	}
}

func userJoin() {
	var newUser user
	for {
		select {
		case newAddress := <-joinChan:
			newUser.address = newAddress
			allUsers = append(allUsers, newUser)
			for _, user := range allUsers {
				io.WriteString(user.address, fmt.Sprintf("User %v has joined the server\n", newAddress))
			}
		}
	}

}

func handler(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		switch fields[0] {
		case "CMD":
			io.WriteString(conn, "LIST: Shows list of categories\nPUB (category name): Publishes message in category subscribers\nSUB(category name): subscribes you to that category\nNICK (name): Changes your nickname\n")
		case "LIST":
			io.WriteString(conn, "List of categories are:\n")
			for k, v := range categories {
				io.WriteString(conn, fmt.Sprintf("%v: %v\n", k, v))
			}
		case "SUB":
			var subArr = make([]interface{}, 2)
			subArr[0] = fields[1]
			subArr[1] = conn
			inRoom := false
			for _, v := range rooms[fields[1]] {
				if v.address == conn {
					inRoom = true
					break
				}
			}
			switch inRoom {
			case true:
				io.WriteString(conn, "You are already subscribed to this channel!\n")
			case false:
				subChannel <- subArr
			}

		case "NICK":
			newName := fields[1]
			for i := 0; i < len(allUsers); i++ {
				if allUsers[i].address == conn {
					allUsers[i].name = newName
					io.WriteString(conn, "Your new name is "+newName+"\n")
					for _, user := range allUsers {
						if user.address != conn {
							io.WriteString(user.address, fmt.Sprintf("User %v has changed their nickname to %v\n", conn, newName))
						}
					}
				}
			}

		case "NEW":
			var newChanArr [2]interface{}
			categoryName := fields[1]
			newChanArr[0] = conn
			newChanArr[1] = categoryName

			newCatChannel <- newChanArr

		case "PUB":
			var msgArr [3]string
			func() {
				for i := 0; i < len(allUsers); i++ {
					if conn == allUsers[i].address {
						switch allUsers[i].name {
						case "":
							msgArr[0] = conn.LocalAddr().String()
						default:
							msgArr[0] = allUsers[i].name
						}

					}
				}
			}()

			message := strings.Join(fields[2:], " ")
			msgArr[1] = fields[1]
			msgArr[2] = message

			newMessage <- msgArr

		}
	}
}

func msgBroadcast() {
	for {
		select {

		case msg := <-newMessage:
			msgString := fmt.Sprintf("%v wrote on channel %v: %v\n", msg[0], msg[1], msg[2])
			roomName := msg[1]
			for _, v := range rooms[roomName] {
				conn := v.address
				io.WriteString(conn, msgString)
			}
		case new := <-newCatChannel:
			var newUser user
			catName := new[1].(string)
			newUser.address = new[0].(net.Conn)
			categories = append(categories, new[1].(string))
			rooms[catName] = make([]user, 0, 10)
			rooms[catName] = append(rooms[catName], newUser)

		case sub := <-subChannel:

			var user user
			roomName := sub[0].(string)
			user.address = sub[1].(net.Conn)

			rooms[roomName] = append(rooms[roomName], user)
			fmt.Println(rooms[roomName])

		}
	}
}

func main() {
	go startTCP()
	go msgBroadcast()
	go userJoin()

	fmt.Scanln()

}
