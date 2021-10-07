package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Delivery struct {
	gorm.Model

	Id                int
	OrderItemId       int
	DeliveredQuantity int
}

type OrderItems struct {
	gorm.Model

	Id           int
	OrderId      int
	PricePerUnit float64
	Quantity     int
	Product      string `gorm:"typevarchar(255)"`
}

type Order struct {
	gorm.Model

	Id         int
	CreatedAt  time.Time
	OrderName  string `gorm:"varchar(255)"`
	CustomerID string `gorm:"varchar(255)"`
}

type CustomerCompanies struct {
	gorm.Model

	CompanyId   int `bson:"company_id"`
	CompanyName string `bson:"company_name"`
}

type Customers struct {
	UserId       string `bson:"user_id"`
	Login        string `bson:"login"`
	Password     int	`bson:"password"`
	Name         string `bson:"name"`
	CompanyId    int	`bson:"company_id"`
	CreditCards  string `bson:"credit_cards"`
}

var db *gorm.DB
var err error
var postgresDB *sql.DB
var mongoDB *mongo.Database

func main() {

	dbURI := fmt.Sprintf("host=%s user=%s dbname=%s sslmode=disable password=%s port=%s",
		"localhost",
		"postgres",
		"store",
		"password",
		"5432")
	mongoDBURI := "mongodb://localhost:27017"

	// openning connection to database
	db, err = gorm.Open(postgres.Open(dbURI), &gorm.Config{})
	postgresDB, err := db.DB()
	if err != nil {
		log.Fatal("Postgres DATABASE CONNECTION NOT WORKING << log.Fatal >>")
	} else {
		fmt.Println("Successfully connected to Postgres database")
	}

	defer postgresDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoDBURI))
	mongoDB = mongoClient.Database("store")

	if err != nil {
		log.Fatal("MongoDB DATABASE CONNECTION NOT WORKING << log.Fatal >>")
	} else {
		fmt.Println("Successfully connected to MongoDB")
	}

	defer mongoClient.Disconnect(ctx)
	// API router
	router := mux.NewRouter()

	router.HandleFunc("/orders", GetOrders).Methods("GET")

	// close connection when function ends

	// server is listening
	http.ListenAndServe(":5000", router)
}

// Api Controllers

func GetOrders(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var orders []Order
	var orderItems []OrderItems
	var customers []Customers
	var customerCompanies []CustomerCompanies
	var jsonString []string

	postgresResult := db.Find(&orders)
	if postgresResult.Error != nil {
		log.Fatal(postgresResult.Error)
	}
	
	postgresResult = db.Find(&orderItems)
	if postgresResult.Error != nil {
		log.Fatal(postgresResult.Error)
	}
	
	customerCursor, err := mongoDB.Collection("customers").Find(ctx, bson.M{})
	if err != nil {
		log.Fatal(err)
	}

	companiesCursor, err := mongoDB.Collection("customer_companies").Find(ctx, bson.M{})
	
	if err != nil {
		log.Fatal(err)
	}
	
	defer customerCursor.Close(ctx)
	defer companiesCursor.Close(ctx)
	
	if err := customerCursor.All(ctx, &customers); err != nil { log.Fatal(err) }
	if err := companiesCursor.All(ctx, &customerCompanies); err != nil { log.Fatal(err) }
	
	for _, order := range orders {
		companiesName := lookUpCustomerCompaniesName(lookUpCustomerCompanyId(order.CustomerID, customers), customerCompanies)
		customerName := lookUpCustomerName(order.CustomerID, customers)
		pricePerUnit, quantity := lookUpPriceAndQuantity(order.Id, orderItems) 

		jsonString = append(jsonString, fmt.Sprintf(`{"OrderName": "%s", "CustomerCompany":"%s", "CustomerName":"%s", "CreatedAt":"%s", "DeliveredAmount":"%f", "TotalAmount":"%f"}`,
						order.OrderName,
						companiesName,
						customerName,
						formatTime(order.CreatedAt),
						pricePerUnit * float64(quantity),
						pricePerUnit * float64(quantity)))
	} 
	// prettyPrint(jsonString)

	jsonArray := make([]json.RawMessage, len(jsonString))
	for i := range jsonString {
		jsonArray[i] = json.RawMessage(jsonString[i])
	}


	w.Header().Set("Access-Control-Allow-Origin", "*") // https://web.dev/cross-origin-resource-sharing/ , https://stackoverflow.com/questions/60287223/cors-issue-when-calling-a-localhost-api-hosted-on-one-port-from-another-port , https://stackoverflow.com/questions/12830095/setting-http-headers
	json.NewEncoder(w).Encode(jsonArray) // https://play.golang.org/p/n5b5ec4297K
}


// util functions

func prettyPrint(input interface{}) {
	b, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Print(string(b))
}


func lookUpCustomerCompaniesName(targetCompanyId interface{}, dataSource []CustomerCompanies) string {
	for _, data := range dataSource {
		if data.CompanyId == targetCompanyId{
			return data.CompanyName
		}
	}

	return ""
}

func lookUpCustomerName(targetCustomerId interface{}, dataSource []Customers) string {
	for _, data := range dataSource {
		if data.UserId == targetCustomerId {
			return data.Name
		}
	}

	return ""
}

func lookUpCustomerCompanyId(targetCustomerId interface{}, dataSource []Customers) int {
	for _, data := range dataSource {
		if data.UserId == targetCustomerId {
			return data.CompanyId
		}
	}

	return -1
}

func lookUpPriceAndQuantity(targetOrderId int, orderItems []OrderItems) (float64, int){
	for _, order := range orderItems {
		if order.OrderId == targetOrderId {
			return order.PricePerUnit, order.Quantity
		}
 	}

	return -1, -1
}

func formatTime(unformattedTime time.Time) string {
 	return unformattedTime.Format("Jan 2th, 3:04 PM")
}