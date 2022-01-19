package jprq

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gosimple/slug"
	"gopkg.in/mgo.v2/bson"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (j *Jprq) WebsocketHandler(w http.ResponseWriter, r *http.Request) {
	tunnel, err := j.GetTunnelByHost(r.Host)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(err.Error()))
		return
	}
	requestMessage := PackageHttpRequest(r)
	rId := requestMessage.ID
	tunnel.requestsTracker.Store(requestMessage.ID, requestMessage.ResponseChan)

	{
		/*
			send the http websocket upgrade request to the client and establish the socket connection in the JPRQ Client side
		*/
		tunnel.requestChan <- requestMessage
	}
	responseMessage, ok := <-requestMessage.ResponseChan
	if !ok {
		return
	}

	// Only pass those headers to the upgrader.
	upgradeHeader := http.Header{}
	{
		if hdr := responseMessage.Header.Get("Sec-Websocket-Protocol"); hdr != "" {
			upgradeHeader.Set("Sec-Websocket-Protocol", hdr)
		}
		if hdr := responseMessage.Header.Get("Set-Cookie"); hdr != "" {
			upgradeHeader.Set("Set-Cookie", hdr)
		}
	}

	ws, err := upgrader.Upgrade(w, r, upgradeHeader)
	conn := &Socket{Conn: ws}
	if err != nil {
		log.Println("Error occurred when creating socket: ", err)
		return
	}
	defer conn.Close()
	keepAlive(conn, time.Second*30)

	PackageWSRequest := func(body []byte, msgType int) RequestMessage {
		requestMessage := RequestMessage{}
		requestMessage.SocketMsgType = msgType
		requestMessage.ID = rId
		requestMessage.Body = body
		return requestMessage

	}
	errCh := make(chan error, 1)
	go func(errCh chan error) {
		for {

			msgType, message, err := conn.ReadMessage()
			if err != nil {
				select {
				case <-tunnel.requestChanCloseNotifier:
					// when tunnel closes requestChan will be closed
					break
				default:
					// signals client that the frontend socket connection has been broken
					tunnel.requestChan <- RequestMessage{
						Status: -1,
						ID:     rId,
					}
				}
				log.Println(err)
				errCh <- err
				break
			}
			requestMessage := PackageWSRequest(message, msgType)
			select {
			case <-tunnel.requestChanCloseNotifier:
				break
			default:
				tunnel.requestChan <- requestMessage

			}

		}
		log.Println("frontend facing goroutine exited")

	}(errCh)
	go func() {
		r, ok := tunnel.requestsTracker.Load(rId)
		if !ok {
			return
		}
		for responseMessage = range r.(chan ResponseMessage) {
			_ = conn.WriteMessage(responseMessage.SocketMsgType, responseMessage.Body)
		}
		_ = conn.WriteMessage(websocket.CloseMessage, []byte(""))
		log.Println("client facing goroutine exited")

	}()
	{
		<-errCh
		rChan, ok := tunnel.requestsTracker.Load(rId) // close client facing goroutine too
		if ok {
			log.Println("closing requestsTracker channnel")
			close(rChan.(chan ResponseMessage))
		}
		tunnel.requestsTracker.Delete(rId) // delete from map as well otherwise channel may be closed twice
	}
}
func (j *Jprq) JPRQClientWebsocketHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("New socket connection request....")
	ws, err := upgrader.Upgrade(w, r, nil)
	conn := &Socket{Conn: ws}
	if err != nil {
		log.Println("Error occurred when creating socket: ", err)
		return
	}
	defer conn.Close()
	keepAlive(conn, time.Minute)

	query := r.URL.Query()
	usernames := query["username"]
	if len(usernames) != 1 {
		log.Println("Websocket Connection: Bad Request: ", query)
		return
	}

	subdomain := usernames[0]
	subdomain = slug.Make(subdomain)
	host := j.GetUnusedHost(fmt.Sprintf("%s.%s", subdomain, j.baseHost), subdomain)
	tunnel := j.AddTunnel(host, conn)
	defer j.DeleteTunnel(tunnel.host)

	message := TunnelMessage{tunnel.host, tunnel.token}
	messageContent, err := bson.Marshal(message)

	conn.WriteMessage(websocket.BinaryMessage, messageContent)

	go tunnel.DispatchRequests()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if _, ok := err.(*websocket.CloseError); ok {
				//websocket.CloseAbnormalClosure is calle when process exits or websocket.close() is called
				fmt.Println("\n\033[31mServer connection closed\033[00m")
			}
			log.Println(err)
			//responseChan <- response // sending this will cause the goroutine to return so no need to close the channel
			break
		}
		response := ResponseMessage{}
		err = bson.Unmarshal(message, &response)
		if err != nil {
			log.Println("Failed to Unmarshal Websocket Message: ", string(message), err)
			continue
		}
		if response.Status == -1 { // for websockets
			r, ok := tunnel.requestsTracker.Load(response.RequestId)
			if !ok {
				continue
			}
			tunnel.requestsTracker.Delete(response.RequestId) // should delete chan from sync map otherwise it will be closed twice
			close(r.(chan ResponseMessage))
			continue
		}
		if response.Token != tunnel.token {
			log.Println("Authentication Failed: ", tunnel.host)
			continue
		}
		r, ok := tunnel.requestsTracker.Load(response.RequestId)
		if !ok {
			log.Println("Request Not Found", response.RequestId)
			continue
		}
		responseChan := r.(chan ResponseMessage)
		tunnel.numOfReqServed++
		responseChan <- response
	}

}
