package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gbasileGP/pubg-leaderboard/internal/store"
	"github.com/gbasileGP/pubg-leaderboard/service"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Server represents the server configuration with a router, a Redis client, a logger, and the leaderboard service.
type Server struct {
	router             *gin.Engine
	redisClient        *store.RedisClient
	leaderboardService *service.LeaderboardService
	logger             *logrus.Logger
}

// NewServer initializes a new server with configured Redis client, leaderboard service, and logger passed from main.
func NewServer(redisClient *store.RedisClient, leaderboardService *service.LeaderboardService, logger *logrus.Logger) *Server {
	router := gin.Default()

	server := &Server{
		router:             router,
		redisClient:        redisClient,
		leaderboardService: leaderboardService,
		logger:             logger,
	}
	server.setupRoutes()

	return server
}

// setupRoutes defines all the routes for the server.
func (s *Server) setupRoutes() {
	s.router.GET("/ping", s.handlePing)
	s.router.GET("/redis-ping", s.handleRedisPing)
	s.router.GET("/current-season", s.handleGetCurrentSeason)
	s.router.GET("/current-leaderboard", s.handleGetCurrentLeaderboard)
	s.router.GET("/player-stats/:playerID", s.handleGetPlayerStats)
	s.router.POST("/backup-leaderboard", s.handleBackupLeaderboard)
	s.router.POST("/restore-leaderboard", s.handleRestoreLeaderboard)
}

// handlePing is a handler for the API health check route.
func (s *Server) handlePing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

// handleRedisPing is a handler for the Redis health check route.
func (s *Server) handleRedisPing(c *gin.Context) {
	err := s.redisClient.Ping(c.Request.Context())
	if err != nil {
		s.logger.WithError(err).Error("Failed to ping Redis")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ping Redis"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

// handleGetCurrentSeason is a handler for fetching the current PUBG season.
func (s *Server) handleGetCurrentSeason(c *gin.Context) {
	seasonData, err := s.leaderboardService.GetCurrentSeason(context.Background())
	if err != nil {
		s.logger.WithError(err).Error("Failed to get current season")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current season"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"seasonData": seasonData})
}

// handleGetCurrentLeaderboard is a handler for fetching the current leaderboard.
func (s *Server) handleGetCurrentLeaderboard(c *gin.Context) {
	leaderboardData, err := s.leaderboardService.GetCurrentLeaderboard(context.Background())
	if err != nil {
		s.logger.WithError(err).Error("Failed to get current leaderboard")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get current leaderboard"})
		return
	}

	// Depending on your LeaderboardData model, you may need to adjust how you marshal the data to JSON
	c.JSON(http.StatusOK, leaderboardData)
}

// handleGetPlayerStats is a handler for fetching the stats of a single player.
func (s *Server) handleGetPlayerStats(c *gin.Context) {
	playerID := c.Param("playerID")

	rank, gamesPlayed, wins, err := s.leaderboardService.GetPlayerStats(context.Background(), playerID)
	if err != nil {
		if err == store.ErrCacheMiss {
			c.JSON(http.StatusNotFound, gin.H{"error": "Player stats not found"})
		} else {
			s.logger.WithError(err).Error("Failed to get player stats")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get player stats"})
		}
		return
	}

	// Send a JSON response with the player stats.
	c.JSON(http.StatusOK, gin.H{
		"playerID":    playerID,
		"rank":        rank,
		"gamesPlayed": gamesPlayed,
		"wins":        wins,
	})
}

// handleBackupLeaderboard handles the request to backup the current leaderboard.
func (s *Server) handleBackupLeaderboard(c *gin.Context) {
	bucketName := "pubg-leaderboard" // This can be a query param or can be set in the config/env.
	backupFileName := fmt.Sprintf("leaderboard_backup_%s.json", time.Now().Format("20060102_150405"))

	err := s.leaderboardService.BackupLeaderboardData(c.Request.Context(), bucketName, backupFileName)
	if err != nil {
		s.logger.WithError(err).Error("API: Failed to backup leaderboard data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to backup leaderboard data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Leaderboard data backed up successfully", "bucket": bucketName, "file": backupFileName})
}

// handleRestoreLeaderboard handles the request to restore the leaderboard from a backup.
func (s *Server) handleRestoreLeaderboard(c *gin.Context) {
	bucketName := "pubg-leaderboard"  // This can be a query param or can be set in the config/env.
	backupFileName := c.Query("file") // The name of the backup file to restore from.

	if backupFileName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Backup file name is required"})
		return
	}

	err := s.leaderboardService.RestoreLeaderboardData(c.Request.Context(), bucketName, backupFileName)
	if err != nil {
		s.logger.WithError(err).Error("API: Failed to restore leaderboard data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore leaderboard data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Leaderboard data restored successfully", "bucket": bucketName, "file": backupFileName})
}

// Run starts the HTTP server on a specific address.
func (s *Server) Run(addr string) error {
	return s.router.Run(addr)
}
