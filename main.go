package main

import (
	"encoding/json"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
)

var incidents []*Incident
var officers []*Officer

var mockStates = ApiResponse{
	Data: &StateResponse{
		Incidents: []*Incident{
			{
				ID:       10,
				CodeName: "code",
				Loc: Location{
					X: 12,
					Y: 123124,
				},
				OfficerId: 5,
			},
		},
		Officers: nil,
	},
	Error: nil,
}

func setupRouter() *gin.Engine {
	// Disable Console Color
	// gin.DisableConsoleColor()
	r := gin.Default()

	// Ping test
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	r.GET("/api/v1/state", func(ctx *gin.Context) {
		//ctx.JSON(http.StatusOK, mockStates)
		ctx.JSON(http.StatusOK, ApiResponse{
			Data: &StateResponse{
				Incidents: incidents,
				Officers:  officers,
			},
			Error: nil,
		})

	})

	return r
}

func main() {
	r := setupRouter()
	ch := StartReceiver()
	defer func() {
		log.Printf("closing receiver")
		ch.Close()
	}()

	// Listen and Server in 0.0.0.0:8080
	r.Run(":8080")

}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func StartReceiver() *amqp.Channel {
	conn, err := amqp.Dial("amqp://localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	//defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")

	q, err := ch.QueueDeclare(
		"events", // name
		false,    // durable
		false,    // delete when unused
		false,    // exclusive
		false,    // no-wait
		nil,      // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
			processIncidentRaw(d.Body)
			marshal, _ := json.Marshal(&StateResponse{
				Incidents: incidents,
				Officers:  officers,
			})
			log.Printf("==> %v \n\n", string(marshal))

		}
	}()

	log.Printf(" Started Receiver")
	return ch
}

func processIncidentRaw(data []byte) {
	var event map[string]interface{}
	err := json.Unmarshal(data, &event)
	if err != nil {
		log.Printf(err.Error())
		return
	}
	eventType, _ := event["type"].(string)
	switch eventType {
	case "IncidentOccurred":
		processIncidentOccurred(event)
	case "IncidentResolved":
		processIncidentResolved(event)
	case "OfficerGoesOnline":
		processOfficerGoesOnline(event)
	case "OfficerLocationUpdated":
		processLocationUpdated(event)
	case "OfficerGoesOffline":
		processOfficerGoesOffline(event)
	}

}

func processOfficerGoesOffline(event map[string]interface{}) {
	offID := int(event["officerId"].(float64))
	officer := findOfficer(offID)
	if officer != nil {
		if officer.Incident != nil {
			officer.Incident.Officer = nil
			officer.Incident.OfficerId = 0
			officer.Incident = nil
		}
		removeOfficer(offID)
	}
}

func removeOfficer(id int) {
	removed := make([]*Officer, 0, len(officers)-1)
	for _, officer := range officers {
		if officer.ID != id {
			removed = append(removed, officer)
		}
	}
	officers = removed
}
func findOfficer(id int) *Officer {
	for _, o := range officers {
		if o.ID == id {
			return o
		}
	}
	return nil
}
func processLocationUpdated(event map[string]interface{}) {
	ofID := int(event["officerId"].(float64))
	var officer = findOfficer(ofID)
	locMap := event["loc"].(map[string]interface{})
	officer.Loc.X = int(locMap["x"].(float64))
	officer.Loc.Y = int(locMap["y"].(float64))
}

func processOfficerGoesOnline(event map[string]interface{}) {
	officer := findOfficer(int(event["officerId"].(float64)))
	if officer == nil {
		officer = &Officer{
			ID:        int(event["officerId"].(float64)),
			BadgeName: event["badgeName"].(string),
		}
		officers = append(officers, officer)
	}

	// assign officer to any available incident
	firstAvailableIncident := findFirstAvailableIncident()
	if firstAvailableIncident != nil {
		assignOfficerToIncident(firstAvailableIncident, officer)
	}
}

func findFirstAvailableIncident() *Incident {
	for _, incident := range incidents {
		if incident.Officer == nil {
			return incident
		}
	}
	return nil
}

func nearestAvailableOfficer(loc Location) *Officer {
	var res *Officer
	for _, officer := range officers {
		if officer.Incident == nil {
			if res == nil || distance(loc, officer.Loc) < distance(loc, res.Loc) {
				res = officer
			}
		}
	}
	return res
}

func findIncidentByID(id int) *Incident {
	for _, incident := range incidents {
		if incident.ID == id {
			return incident
		}
	}
	return nil
}

func distance(loc1, loc2 Location) float64 {
	a := loc1.X - loc2.X
	b := loc1.Y - loc2.Y
	return math.Sqrt(float64(a*a + b*b))
}

func processIncidentResolved(event map[string]interface{}) {
	incident := findIncidentByID(int(event["incidentId"].(float64)))

	//unassign officer
	incident.Officer.Incident = nil

	removeIncident(incident)
}

func removeIncident(incident *Incident) {
	removed := make([]*Incident, 0, len(incidents)-1)
	for _, in := range incidents {
		if in.ID != incident.ID {
			removed = append(removed, in)
		}
	}
	incidents = removed
}

func processIncidentOccurred(event map[string]interface{}) {
	var s struct {
		IncidentID int      `json:"incidentId"`
		CodeName   string   `json:"codeName"`
		Loc        Location `json:"loc"`
	}

	marshal, _ := json.Marshal(event)
	err := json.Unmarshal(marshal, &s)
	incident := &Incident{
		ID:       s.IncidentID,
		CodeName: s.CodeName,
		Loc:      s.Loc,
	}
	if err != nil {
		log.Printf(err.Error())
		return
	}

	// assign officer

	officer := nearestAvailableOfficer(incident.Loc)
	if officer != nil {
		assignOfficerToIncident(incident, officer)
	}

	incidents = append(incidents, incident)
}

func assignOfficerToIncident(incident *Incident, officer *Officer) {
	incident.OfficerId = officer.ID
	incident.Officer = officer
	officer.Incident = incident
}
