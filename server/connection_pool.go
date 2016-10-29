package server

import "gopkg.in/mgo.v2"

// ConnectionPool - is pool of mongo connections
type ConnectionPool struct {
}

var connectionPoolInstance *ConnectionPool

// NewConnectionPool - creates new mongo connection pool
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{}
}

// GetConnection - get another free connection to mgo. Also it selects database for usage
func (c *ConnectionPool) GetConnection() (*mgo.Database, error) {
	session, err := mgo.Dial("mongodb")
	if err != nil {
		return nil, err
	}

	return session.DB("testdatabase"), nil
}
