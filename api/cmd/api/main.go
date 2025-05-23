package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/okteto/movies/pkg/database"

	"fmt"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
)

var db *sql.DB

func main() {
	db = database.Open()
	defer db.Close()

	if len(os.Args) > 1 && os.Args[1] == "load-data" {
		database.Ping(db)
		fmt.Println("Loading data...")
		loadData()
		return
	}

	fmt.Println("Running server on port 8080...")
	handleRequests()
}

type Rental struct {
	Movie string
	Price string
}

type Movie struct {
	ID            int     `json:"id,omitempty"`
	VoteAverage   float64 `json:"vote_average,omitempty"`
	OriginalTitle string  `json:"original_title,omitempty"`
	BackdropPath  string  `json:"backdrop_path,omitempty"`
	Price         float64 `json:"price,omitempty"`
	Overview      string  `json:"overview,omitempty"`
}

type User struct {
	Userid    int
	Firstname string
	Lastname  string
	Phone     string
	City      string
	State     string
	Zip       string
	Age       int
	Gender    string
}

func loadData() {
	dropTableStmt := `DROP TABLE IF EXISTS users`
	if _, err := db.Exec(dropTableStmt); err != nil {
		log.Panic(err)
	}

	createTableStmt := `CREATE TABLE IF NOT EXISTS users (user_id int NOT NULL UNIQUE, first_name varchar(255), last_name varchar(255), phone varchar(15), city varchar(255), state varchar(30), zip varchar(12), age int, gender varchar(10))`
	if _, err := db.Exec(createTableStmt); err != nil {
		log.Panic(err)
	}

	jsonContent, err := os.ReadFile("data/users.json")
	if err != nil {
		log.Panic(err)
	}

	var users []User

	unmarshalErr := json.Unmarshal([]byte(jsonContent), &users)

	if unmarshalErr != nil {
		log.Panic(err)
	}

	for _, user := range users {
		insertStmt := `insert into "users"("user_id", "first_name", "last_name", "phone", "city", "state", "zip", "age", "gender") values($1, $2, $3, $4, $5, $6, $7, $8, $9)`
		if _, err := db.Exec(insertStmt, user.Userid, user.Firstname, user.Lastname, user.Phone, user.City, user.State, user.Zip, user.Age, user.Gender); err != nil {
			log.Panic(err)
		}
	}

	return
}

func handleRequests() {
	// Set up OpenTelemetry propagator for baggage
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	// Create a new router with OpenTelemetry instrumentation
	muxRouter := mux.NewRouter().StrictSlash(true)

	// Add OpenTelemetry middleware to the router
	muxRouter.Use(otelmux.Middleware("api-service"))

	muxRouter.HandleFunc("/rentals", rentals)
	muxRouter.HandleFunc("/users", allUsers)
	muxRouter.HandleFunc("/users/{userid}", singleUser)

	// Create a custom HTTP client with OpenTelemetry instrumentation for outgoing requests
	http.DefaultClient = &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	// Log when the server starts
	fmt.Println("Server running on port 8080 with OpenTelemetry baggage propagation enabled")
	log.Fatal(http.ListenAndServe(":8080", muxRouter))
}

func rentals(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received request...")

	// Log baggage from the request context
	b := baggage.FromContext(r.Context())
	if len(b.Members()) > 0 {
		fmt.Printf("Received request with baggage: %v\n", b)
	}

	rows, err := db.Query("SELECT * FROM rentals")
	if err != nil {
		fmt.Println("error listing rentals", err)
		w.WriteHeader(500)
		return
	}
	defer rows.Close()

	var rentals []Rental

	for rows.Next() {
		var r Rental
		if err := rows.Scan(&r.Movie, &r.Price); err != nil {
			fmt.Println("error scanning row", err)
			os.Exit(1)
		}
		rentals = append(rentals, r)
	}
	if err = rows.Err(); err != nil {
		fmt.Println("error in rows", err)
		os.Exit(1)
	}

	// Create a new request with the original request's context
	// OpenTelemetry will automatically propagate the baggage
	req, err := http.NewRequestWithContext(r.Context(), "GET", "http://catalog:8080/catalog", nil)
	if err != nil {
		fmt.Println("error creating catalog request", err)
		w.WriteHeader(500)
		return
	}

	fmt.Println("Making request to catalog service with baggage propagation")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("error listing catalog", err)
		w.WriteHeader(500)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("error reading catalog", err)
		w.WriteHeader(500)
		return
	}

	movies := []Movie{}
	if err := json.Unmarshal(body, &movies); err != nil {
		fmt.Println("error unmarshaling catalog", err)
		w.WriteHeader(500)
		return
	}

	result := []Movie{}
	for _, rental := range rentals {
		for _, m := range movies {
			if rental.Movie == strconv.Itoa(m.ID) {
				price, _ := strconv.ParseFloat(rental.Price, 64)
				m.Price = price
				result = append(result, m)
			}
		}
	}

	fmt.Println("Returned", result)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func allUsers(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received request...")

	rows, err := db.Query("SELECT * FROM users")
	if err != nil {
		fmt.Println("error listing users", err)
		w.WriteHeader(500)
		return
	}
	defer rows.Close()

	var users []User

	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Userid, &u.Firstname, &u.Lastname, &u.Phone, &u.City, &u.State, &u.Zip, &u.Age, &u.Gender); err != nil {
			log.Panic("error scanning row", err)
		}
		users = append(users, u)
	}
	if err = rows.Err(); err != nil {
		log.Panic("error in rows", err)
	}

	fmt.Println("Returned", len(users), "user records.")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func singleUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userid := vars["userid"]

	fmt.Println("Received request...")

	row := db.QueryRow("SELECT * FROM users WHERE user_id = $1", userid)

	var user User

	if err := row.Scan(&user.Userid, &user.Firstname, &user.Lastname, &user.Phone, &user.City, &user.State, &user.Zip, &user.Age, &user.Gender); err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No user was found")
			w.WriteHeader(404)
			return
		} else {
			log.Panic("error scanning returned user", err)
		}
	}

	fmt.Println("Returned", user)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
