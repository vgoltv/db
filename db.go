package db

import (
	"database/sql"
	"errors"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"strconv"
	"strings"
)

type Host struct {
	db     *sql.DB
	dbname string
}

// Common for each of the db datastructures used here
type dbDatastructure struct {
	host  *Host
	table string
}

type (
	List     dbDatastructure
	Set      dbDatastructure
	HashMap  dbDatastructure
	KeyValue dbDatastructure
)

const (
	// Version number. Stable API within major version numbers.
	Version = 1.0
	// The default "username:password@host:port/database" that the database is running at
	defaultDatabaseServer = ""     // "username:password@server:port/"
	defaultDatabaseName   = "test" // "main"
	defaultStringLength   = 42     // using VARCHAR, so this will be expanded up to 65535 characters as needed, unless mysql strict mode is enabled
	defaultPort           = 3306
)

/* --- Helper functions --- */

// Test if the local database server is up and running
func TestConnection() (err error) {
	return TestConnectionHost(defaultDatabaseServer)
}

// Test if a given database server at host:port is up and running.
// Also pings.
func TestConnectionHost(connectionString string) (err error) {
	newConnectionString, _ := rebuildConnectionString(connectionString)
	// Connect to the given host:port
	db, err := sql.Open("mysql", newConnectionString)
	defer db.Close()
	err = db.Ping()
	if err != nil {
		log.Println("Ping: failed")
	} else {
		log.Println("Ping: ok")
	}
	return err
}

// Split a string into two parts, given a delimiter.
// Returns the two parts and true if it works out.
func twoFields(s, delim string) (string, string, bool) {
	if strings.Count(s, delim) != 1 {
		return s, "", false
	}
	fields := strings.Split(s, delim)
	return fields[0], fields[1], true
}

/* --- Host functions --- */

func leftOf(s, delim string) string {
	if left, _, ok := twoFields(s, delim); ok {
		return strings.TrimSpace(left)
	}
	return ""
}

func rightOf(s, delim string) string {
	if _, right, ok := twoFields(s, delim); ok {
		return strings.TrimSpace(right)
	}
	return ""
}

// Parse a DSN
func splitConnectionString(connectionString string) (string, string, bool, string, string, string) {
	var (
		userPass, hostPortDatabase, dbname       string
		hostPort, password, username, port, host string
		hasPassword                              bool
	)

	// Gather the fields

	// Optional left side of @ with username and password
	userPass = leftOf(connectionString, "@")
	if userPass != "" {
		hostPortDatabase = rightOf(connectionString, "@")
	} else {
		if strings.HasSuffix(connectionString, "@") {
			hostPortDatabase = connectionString[:len(connectionString)-1]
		} else {
			hostPortDatabase = connectionString
		}
	}
	// Optional right side of / with database name
	dbname = rightOf(hostPortDatabase, "/")
	if dbname != "" {
		hostPort = leftOf(hostPortDatabase, "/")
	} else {
		if strings.HasSuffix(hostPortDatabase, "/") {
			hostPort = hostPortDatabase[:len(hostPortDatabase)-1]
		} else {
			hostPort = hostPortDatabase
		}
		dbname = defaultDatabaseName
	}
	// Optional right side of : with password
	password = rightOf(userPass, ":")
	if password != "" {
		username = leftOf(userPass, ":")
	} else {
		if strings.HasSuffix(userPass, ":") {
			username = userPass[:len(userPass)-1]
			hasPassword = true
		} else {
			username = userPass
		}
	}
	// Optional right side of : with port
	port = rightOf(hostPort, ":")
	if port != "" {
		host = leftOf(hostPort, ":")
	} else {
		if strings.HasSuffix(hostPort, ":") {
			host = hostPort[:len(hostPort)-1]
		} else {
			host = hostPort
		}
		if host != "" {
			port = strconv.Itoa(defaultPort)
		}
	}

	log.Println("Connection:")
	log.Println("\tusername:\t", username)
	log.Println("\tpassword:\t", password)
	log.Println("\thas password:\t", hasPassword)
	log.Println("\thost:\t\t", host)
	log.Println("\tport:\t\t", port)
	log.Println("\tdbname:\t\t", dbname)
	log.Println()

	return username, password, hasPassword, host, port, dbname
}

// Build a DSN
func buildConnectionString(username, password string, hasPassword bool, host, port, dbname string) string {

	// Build the new connection string

	newConnectionString := ""
	if (host != "") && (port != "") {
		newConnectionString += "tcp(" + host + ":" + port + ")"
	} else if host != "" {
		newConnectionString += "tcp(" + host + ")"
	} else if port != "" {
		newConnectionString += "tcp(" + ":" + port + ")"
		log.Fatalln("There is only a port. This should not happen.")
	}
	if (username != "") && hasPassword {
		newConnectionString = username + ":" + password + "@" + newConnectionString
	} else if username != "" {
		newConnectionString = username + "@" + newConnectionString
	} else if hasPassword {
		newConnectionString = ":" + password + "@" + newConnectionString
	}
	newConnectionString += "/"

	log.Println("DSN:", newConnectionString)

	return newConnectionString
}

// Take apart and rebuild the connection string. Also return the dbname.
func rebuildConnectionString(connectionString string) (string, string) {
	username, password, hasPassword, hostname, port, dbname := splitConnectionString(connectionString)
	return buildConnectionString(username, password, hasPassword, hostname, port, dbname), dbname
}

// Create a new database connection.
// connectionString may be on the form "username:password@host:port/database".
func NewHost(connectionString string) *Host {

	newConnectionString, dbname := rebuildConnectionString(connectionString)

	db, err := sql.Open("mysql", newConnectionString)
	if err != nil {
		log.Fatalln("Could not connect to " + newConnectionString + "!")
	}
	host := &Host{db, dbname}
	if err := db.Ping(); err != nil {
		log.Fatalln("Database does not reply to ping: " + err.Error())
	}
	if err := host.createDatabase(); err != nil {
		log.Fatalln("Could not create database " + host.dbname + ": " + err.Error())
	}
	if err := host.useDatabase(); err != nil {
		panic("Could not use database " + host.dbname + ": " + err.Error())
	}
	return host
}

// The default database connection
func New() *Host {
	connectionString := defaultDatabaseServer + defaultDatabaseName
	if !strings.HasSuffix(defaultDatabaseServer, "/") {
		connectionString = defaultDatabaseServer + "/" + defaultDatabaseName
	}
	return NewHost(connectionString)
}

// Select a different database. Create the database if needed.
func (host *Host) SelectDatabase(dbname string) error {
	host.dbname = dbname
	if err := host.createDatabase(); err != nil {
		return err
	}
	if err := host.useDatabase(); err != nil {
		return err
	}
	return nil
}

// Will create the database if it does not already exist.
func (host *Host) createDatabase() error {
	if _, err := host.db.Exec("CREATE DATABASE IF NOT EXISTS " + host.dbname + " CHARACTER SET = utf8"); err != nil {
		return err
	}
	log.Println("Created database " + host.dbname)
	return nil
}

// Use the host.dbname database.
func (host *Host) useDatabase() error {
	if _, err := host.db.Exec("USE " + host.dbname); err != nil {
		return err
	}
	log.Println("Using database " + host.dbname)
	return nil
}

// Close the connection.
func (host *Host) Close() {
	host.db.Close()
}

/* --- List functions --- */

// Create a new list. Lists are ordered.
func NewList(host *Host, table string) *List {
	l := &List{host, table}
	// list is the name of the column
	if _, err := l.host.db.Exec("CREATE TABLE IF NOT EXISTS " + table + " (id INT PRIMARY KEY AUTO_INCREMENT, list VARCHAR(" + strconv.Itoa(defaultStringLength) + "))"); err != nil {
		// This is more likely to happen at the start of the program,
		// hence the panic.
		panic("Could not create table " + table + ": " + err.Error())
	}
	log.Println("Created table " + table + " in database " + host.dbname)
	return l
}

// Add an element to the list
func (rl *List) Add(value string) error {
	// list is the name of the column
	_, err := rl.host.db.Exec("INSERT INTO "+rl.table+" (list) VALUES (?)", value)
	return err
}

// Get all elements of a list
func (rl *List) GetAll() ([]string, error) {
	rows, err := rl.host.db.Query("SELECT list FROM " + rl.table + " ORDER BY id")
	if err != nil {
		panic(err.Error())
	}
	defer rows.Close()
	var (
		values []string
		value  string
	)
	for rows.Next() {
		err = rows.Scan(&value)
		values = append(values, value)
		if err != nil {
			panic(err.Error())
		}
	}
	if err := rows.Err(); err != nil {
		panic(err.Error())
	}
	return values, nil
}

// Get the last element of a list
func (rl *List) GetLast() (string, error) {
	// Fetches the item with the largest id.
	// Faster than "ORDER BY id DESC limit 1" for large tables.
	rows, err := rl.host.db.Query("SELECT list FROM " + rl.table + " WHERE id = (SELECT MAX(id) FROM " + rl.table + ")")
	if err != nil {
		panic(err.Error())
	}
	defer rows.Close()
	var value string
	// Get the value. Will only loop once.
	for rows.Next() {
		err = rows.Scan(&value)
		if err != nil {
			panic(err.Error())
		}
	}
	if err := rows.Err(); err != nil {
		panic(err.Error())
	}
	return value, nil
}

// Get the last N elements of a list
func (rl *List) GetLastN(n int) ([]string, error) {
	// TODO: http://stackoverflow.com/a/574148/131264 instead of GetAll()
	values, err := rl.GetAll()
	if err != nil {
		return []string{}, err
	}
	if len(values) < n {
		return []string{}, errors.New("Too few elements in table at GetLastN")
	}
	return values[len(values)-n:], nil
}

// Remove this list
func (rl *List) Remove() error {
	// Remove the table
	_, err := rl.host.db.Exec("DROP TABLE " + rl.table)
	return err
}

// Clear the list contents
func (rl *List) Clear() error {
	// Clear the table
	_, err := rl.host.db.Exec("TRUNCATE TABLE " + rl.table)
	return err
}

/* --- Set functions --- */

//// Create a new set
//func NewSet(host *sql.DB, table string) *Set {
//	return &Set{host, table, defaultDatabaseName}
//}
//
//// Select a different database
//func (rs *Set) SelectDatabase(dbname string) {
//	rs.dbname = dbname
//}
//
//// Add an element to the set
//func (rs *Set) Add(value string) error {
//	db := rs.host.Get(rs.dbname)
//	_, err := db.Do("SADD", rs.table, value)
//	return err
//}
//
//// Check if a given value is in the set
//func (rs *Set) Has(value string) (bool, error) {
//	db := rs.host.Get(rs.dbname)
//	retval, err := db.Do("SISMEMBER", rs.table, value)
//	if err != nil {
//		panic(err)
//	}
//	return db.Bool(retval, err)
//}
//
//// Get all elements of the set
//func (rs *Set) GetAll() ([]string, error) {
//	db := rs.host.Get(rs.dbname)
//	result, err := db.Values(db.Do("SMEMBERS", rs.table))
//	strs := make([]string, len(result))
//	for i := 0; i < len(result); i++ {
//		strs[i] = getString(result, i)
//	}
//	return strs, err
//}
//
//// Remove an element from the set
//func (rs *Set) Del(value string) error {
//	db := rs.host.Get(rs.dbname)
//	_, err := db.Do("SREM", rs.table, value)
//	return err
//}
//
//// Remove this set
//func (rs *Set) Remove() error {
//	db := rs.host.Get(rs.dbname)
//	_, err := db.Do("DEL", rs.table)
//	return err
//}
//
///* --- HashMap functions --- */
//
//// Create a new hashmap
//func NewHashMap(host *sql.DB, table string) *HashMap {
//	return &HashMap{host, table, defaultDatabaseName}
//}
//
//// Select a different database
//func (rh *HashMap) SelectDatabase(dbname string) {
//	rh.dbname = dbname
//}
//
//// Set a value in a hashmap given the element id (for instance a user id) and the key (for instance "password")
//func (rh *HashMap) Set(elementid, key, value string) error {
//	db := rh.host.Get(rh.dbname)
//	_, err := db.Do("HSET", rh.table+":"+elementid, key, value)
//	return err
//}
//
//// Get a value from a hashmap given the element id (for instance a user id) and the key (for instance "password")
//func (rh *HashMap) Get(elementid, key string) (string, error) {
//	db := rh.host.Get(rh.dbname)
//	result, err := db.String(db.Do("HGET", rh.table+":"+elementid, key))
//	if err != nil {
//		return "", err
//	}
//	return result, nil
//}
//
//// Check if a given elementid + key is in the hash map
//func (rh *HashMap) Has(elementid, key string) (bool, error) {
//	db := rh.host.Get(rh.dbname)
//	retval, err := db.Do("HEXISTS", rh.table+":"+elementid, key)
//	if err != nil {
//		panic(err)
//	}
//	return db.Bool(retval, err)
//}
//
//// Check if a given elementid exists as a hash map at all
//func (rh *HashMap) Exists(elementid string) (bool, error) {
//	// TODO: key is not meant to be a wildcard, check for "*"
//	return hasKey(rh.host, rh.table+":"+elementid, rh.dbname)
//}
//
//// Get all elementid's for all hash elements
//func (rh *HashMap) GetAll() ([]string, error) {
//	db := rh.host.Get(rh.dbname)
//	result, err := db.Values(db.Do("KEYS", rh.table+":*"))
//	strs := make([]string, len(result))
//	idlen := len(rh.table)
//	for i := 0; i < len(result); i++ {
//		strs[i] = getString(result, i)[idlen+1:]
//	}
//	return strs, err
//}
//
//// Remove a key for an entry in a hashmap (for instance the email field for a user)
//func (rh *HashMap) DelKey(elementid, key string) error {
//	db := rh.host.Get(rh.dbname)
//	_, err := db.Do("HDEL", rh.table+":"+elementid, key)
//	return err
//}
//
//// Remove an element (for instance a user)
//func (rh *HashMap) Del(elementid string) error {
//	db := rh.host.Get(rh.dbname)
//	_, err := db.Do("DEL", rh.table+":"+elementid)
//	return err
//}
//
//// Remove this hashmap
//func (rh *HashMap) Remove() error {
//	db := rh.host.Get(rh.dbname)
//	_, err := db.Do("DEL", rh.table)
//	return err
//}
//
///* --- KeyValue functions --- */
//
//// Create a new key/value
//func NewKeyValue(host *sql.DB, table string) *KeyValue {
//	return &KeyValue{host, table, defaultDatabaseName}
//}
//
//// Select a different database
//func (rkv *KeyValue) SelectDatabase(dbname string) {
//	rkv.dbname = dbname
//}
//
//// Set a key and value
//func (rkv *KeyValue) Set(key, value string) error {
//	db := rkv.host.Get(rkv.dbname)
//	_, err := db.Do("SET", rkv.table+":"+key, value)
//	return err
//}
//
//// Get a value given a key
//func (rkv *KeyValue) Get(key string) (string, error) {
//	db := rkv.host.Get(rkv.dbname)
//	result, err := db.String(db.Do("GET", rkv.table+":"+key))
//	if err != nil {
//		return "", err
//	}
//	return result, nil
//}
//
//// Remove a key
//func (rkv *KeyValue) Del(key string) error {
//	db := rkv.host.Get(rkv.dbname)
//	_, err := db.Do("DEL", rkv.table+":"+key)
//	return err
//}
//
//// Remove this key/value
//func (rkv *KeyValue) Remove() error {
//	db := rkv.host.Get(rkv.dbname)
//	_, err := db.Do("DEL", rkv.table)
//	return err
//}
//
//// --- Generic db functions ---
//
//// Check if a key exists. The key can be a wildcard (ie. "user*").
//func hasKey(host *sql.DB, wildcard string, dbname string) (bool, error) {
//	db := host.Get(dbname)
//	result, err := db.Values(db.Do("KEYS", wildcard))
//	if err != nil {
//		return false, err
//	}
//	return len(result) > 0, nil
//}
