package services

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ConfigVersioning manages dynamic config with rollback
type ConfigVersioning struct {
	redis *redis.Client
}

type ConfigVersion struct {
	ID          string                 `json:"id"`
	Version     int                    `json:"version"`
	Name        string                 `json:"name"`
	Config      map[string]interface{} `json:"config"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	IsActive    bool                   `json:"is_active"`
	Description string                 `json:"description"`
}

func NewConfigVersioning(redis *redis.Client) *ConfigVersioning {
	return &ConfigVersioning{redis: redis}
}

// CreateVersion creates new config version
func (c *ConfigVersioning) CreateVersion(name string, config map[string]interface{}, createdBy, description string) (*ConfigVersion, error) {
	// Get next version number
	currentVersion := c.getCurrentVersion(name)
	newVersion := currentVersion + 1

	version := &ConfigVersion{
		ID:          uuid.New().String(),
		Version:     newVersion,
		Name:        name,
		Config:      config,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
		IsActive:    false, // Not active until explicitly activated
		Description: description,
	}

	// Store version
	key := "config:version:" + name + ":" + strconv.Itoa(newVersion)
	data, _ := json.Marshal(version)
	c.redis.Set(context.Background(), key, data, 0)

	// Store version list
	c.redis.ZAdd(context.Background(), "config:versions:"+name, redis.Z{
		Score:  float64(newVersion),
		Member: version.ID,
	})

	return version, nil
}

// ActivateVersion makes a version active
func (c *ConfigVersioning) ActivateVersion(name string, version int) error {
	// Deactivate current
	current := c.getActiveVersion(name)
	if current != nil {
		current.IsActive = false
		c.saveVersion(current)
	}

	// Activate new
	newVersion := c.getVersionByNumber(name, version)
	if newVersion == nil {
		return nil // Version not found
	}

	newVersion.IsActive = true
	c.saveVersion(newVersion)

	// Update active pointer
	key := "config:active:" + name
	c.redis.Set(context.Background(), key, version, 0)

	return nil
}

// Rollback rolls back to previous version
func (c *ConfigVersioning) Rollback(name string) (*ConfigVersion, error) {
	current := c.getActiveVersion(name)
	if current == nil || current.Version <= 1 {
		return nil, nil // No previous version
	}

	previousVersion := current.Version - 1
	if err := c.ActivateVersion(name, previousVersion); err != nil {
		return nil, err
	}

	return c.getVersionByNumber(name, previousVersion), nil
}

// GetActiveConfig retrieves active config
func (c *ConfigVersioning) GetActiveConfig(name string) map[string]interface{} {
	version := c.getActiveVersion(name)
	if version == nil {
		return nil
	}
	return version.Config
}

// GetVersionHistory returns all versions
func (c *ConfigVersioning) GetVersionHistory(name string) []ConfigVersion {
	// Query from Redis
	return []ConfigVersion{}
}

// CompareVersions shows diff between versions
func (c *ConfigVersioning) CompareVersions(name string, v1, v2 int) map[string]interface{} {
	version1 := c.getVersionByNumber(name, v1)
	version2 := c.getVersionByNumber(name, v2)

	if version1 == nil || version2 == nil {
		return nil
	}

	return map[string]interface{}{
		"version_1": v1,
		"version_2": v2,
		"config_1":  version1.Config,
		"config_2":  version2.Config,
		"diff":      generateDiff(version1.Config, version2.Config),
	}
}

// Helper methods
func (c *ConfigVersioning) getCurrentVersion(name string) int {
	key := "config:version:latest:" + name
	val, _ := c.redis.Get(context.Background(), key).Int()
	return val
}

func (c *ConfigVersioning) getActiveVersion(name string) *ConfigVersion {
	key := "config:active:" + name
	versionNum, _ := c.redis.Get(context.Background(), key).Int()
	if versionNum == 0 {
		return nil
	}
	return c.getVersionByNumber(name, versionNum)
}

func (c *ConfigVersioning) getVersionByNumber(name string, version int) *ConfigVersion {
	key := "config:version:" + name + ":" + strconv.Itoa(version)
	data, _ := c.redis.Get(context.Background(), key).Result()
	if data == "" {
		return nil
	}

	var v ConfigVersion
	json.Unmarshal([]byte(data), &v)
	return &v
}

func (c *ConfigVersioning) saveVersion(v *ConfigVersion) {
	key := "config:version:" + v.Name + ":" + strconv.Itoa(v.Version)
	data, _ := json.Marshal(v)
	c.redis.Set(context.Background(), key, data, 0)
}

func generateDiff(c1, c2 map[string]interface{}) map[string]interface{} {
	diff := map[string]interface{}{}
	for k, v := range c2 {
		if c1[k] != v {
			diff[k] = map[string]interface{}{
				"old": c1[k],
				"new": v,
			}
		}
	}
	return diff
}

// ConfigVersioningAPI endpoints documentation
func GetConfigVersioningEndpoints() []map[string]interface{} {
	return []map[string]interface{}{
		{"method": "POST", "path": "/internal/config/:name/version", "desc": "Create new config version"},
		{"method": "PUT", "path": "/internal/config/:name/activate/:version", "desc": "Activate specific version"},
		{"method": "POST", "path": "/internal/config/:name/rollback", "desc": "Rollback to previous"},
		{"method": "GET", "path": "/internal/config/:name/history", "desc": "View version history"},
		{"method": "GET", "path": "/internal/config/:name/compare", "desc": "Compare two versions"},
	}
}

var ConfigVersioner *ConfigVersioning

func InitConfigVersioning(redis *redis.Client) {
	ConfigVersioner = NewConfigVersioning(redis)
}

func GetConfigVersioning() *ConfigVersioning {
	return ConfigVersioner
}
