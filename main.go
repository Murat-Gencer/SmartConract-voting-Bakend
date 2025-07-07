package main

import (
	"log"
	"voting-backend/functions"
	"github.com/gin-gonic/gin"
)

const (
	DatabaseUser     = "admin"
	DatabasePassword = "admin"
	DatabaseName     = "voting_db"
	DatabaseHost     = "localhost"
	DatabasePort     = 5432
	ServerPort       = ":8080"
)

func setupRoutes() *gin.Engine {
	router := gin.Default()

	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	apiV1 := router.Group("/api/v1")
	{
		apiV1.POST("/createPoll", functions.CreatePoll)
		apiV1.POST("/getPolls", functions.ListPolls)
		apiV1.POST("/polls/:id/vote", functions.CastVote)
		apiV1.POST("/userVotes", functions.GetUserVotes)
	}

	return router
}

func main() {
	functions.Init()

	router := setupRoutes()

	log.Printf("Server starting on port %s", ServerPort)
	log.Fatal(router.Run(ServerPort))
}
