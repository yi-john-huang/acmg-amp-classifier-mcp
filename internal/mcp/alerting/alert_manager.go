package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type AlertManager struct {
	config     AlertConfig
	logger     *logrus.Logger
	channels   map[string]AlertChannel
	rules      []AlertRule
	alerts     map[string]*Alert
	history    []Alert
	mutex      sync.RWMutex
	stopChan   chan struct{}
	ticker     *time.Ticker
	rateLimit  map[string]time.Time
	silences   map[string]AlertSilence
}

type AlertConfig struct {
	EnableAlerting      bool          `json:"enable_alerting"`
	EvaluationInterval  time.Duration `json:"evaluation_interval"`
	MaxAlertHistory     int           `json:"max_alert_history"`
	DefaultSeverity     string        `json:"default_severity"`
	RateLimitDuration   time.Duration `json:"rate_limit_duration"`
	RetryAttempts       int           `json:"retry_attempts"`
	RetryDelay          time.Duration `json:"retry_delay"`
	GlobalSilenceWindow time.Duration `json:"global_silence_window"`
}

type Alert struct {
	ID          string                 `json:"id"`
	RuleName    string                 `json:"rule_name"`
	Severity    AlertSeverity          `json:"severity"`
	Status      AlertStatus            `json:"status"`
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
	Value       float64                `json:"value"`
	Threshold   float64                `json:"threshold"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	Count       int                    `json:"count"`
	LastSent    *time.Time             `json:"last_sent,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type AlertRule struct {
	Name        string                 `json:"name"`
	Query       string                 `json:"query"`
	Condition   AlertCondition         `json:"condition"`
	Threshold   float64                `json:"threshold"`
	Duration    time.Duration          `json:"duration"`
	Severity    AlertSeverity          `json:"severity"`
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
	Channels    []string               `json:"channels"`
	Enabled     bool                   `json:"enabled"`
	LastFired   *time.Time             `json:"last_fired,omitempty"`
	FireCount   int                    `json:"fire_count"`
}

type AlertCondition struct {
	Operator  ConditionOperator `json:"operator"`
	Value     float64           `json:"value"`
	Duration  time.Duration     `json:"duration"`
	WindowSize time.Duration    `json:"window_size,omitempty"`
}

type AlertSilence struct {
	ID        string            `json:"id"`
	Matchers  map[string]string `json:"matchers"`
	StartsAt  time.Time         `json:"starts_at"`
	EndsAt    time.Time         `json:"ends_at"`
	CreatedBy string            `json:"created_by"`
	Comment   string            `json:"comment"`
}

type AlertChannel interface {
	Name() string
	Send(ctx context.Context, alert *Alert) error
	Test(ctx context.Context) error
}

type AlertSeverity string
type AlertStatus string
type ConditionOperator string

const (
	SeverityCritical AlertSeverity = "critical"
	SeverityHigh     AlertSeverity = "high"
	SeverityWarning  AlertSeverity = "warning"
	SeverityInfo     AlertSeverity = "info"

	StatusPending  AlertStatus = "pending"
	StatusFiring   AlertStatus = "firing"
	StatusResolved AlertStatus = "resolved"
	StatusSilenced AlertStatus = "silenced"

	OperatorGreaterThan    ConditionOperator = "gt"
	OperatorGreaterEqual   ConditionOperator = "gte"
	OperatorLessThan       ConditionOperator = "lt"
	OperatorLessEqual      ConditionOperator = "lte"
	OperatorEqual          ConditionOperator = "eq"
	OperatorNotEqual       ConditionOperator = "neq"
)

// Pre-defined alert rules for MCP server monitoring
var DefaultAlertRules = []AlertRule{
	{
		Name:        "HighToolFailureRate",
		Query:       "tool_failure_rate",
		Condition:   AlertCondition{Operator: OperatorGreaterThan, Value: 10.0, Duration: 5 * time.Minute},
		Threshold:   10.0,
		Duration:    5 * time.Minute,
		Severity:    SeverityCritical,
		Summary:     "High tool failure rate detected",
		Description: "MCP tool failure rate is above {{ .Threshold }}% for {{ .Duration }}",
		Labels:      map[string]string{"service": "mcp-server", "component": "tools"},
		Channels:    []string{"default"},
		Enabled:     true,
	},
	{
		Name:        "DatabaseConnectionIssues",
		Query:       "database_connection_failures",
		Condition:   AlertCondition{Operator: OperatorGreaterThan, Value: 0, Duration: 1 * time.Minute},
		Threshold:   0,
		Duration:    1 * time.Minute,
		Severity:    SeverityHigh,
		Summary:     "Database connection failures detected",
		Description: "Database connections are failing for {{ .Duration }}",
		Labels:      map[string]string{"service": "mcp-server", "component": "database"},
		Channels:    []string{"default"},
		Enabled:     true,
	},
	{
		Name:        "HighMemoryUsage",
		Query:       "memory_usage_percent",
		Condition:   AlertCondition{Operator: OperatorGreaterThan, Value: 85.0, Duration: 10 * time.Minute},
		Threshold:   85.0,
		Duration:    10 * time.Minute,
		Severity:    SeverityWarning,
		Summary:     "High memory usage",
		Description: "Memory usage is above {{ .Threshold }}% for {{ .Duration }}",
		Labels:      map[string]string{"service": "mcp-server", "component": "system"},
		Channels:    []string{"default"},
		Enabled:     true,
	},
	{
		Name:        "SlowToolExecution",
		Query:       "tool_execution_time_p95",
		Condition:   AlertCondition{Operator: OperatorGreaterThan, Value: 30.0, Duration: 5 * time.Minute},
		Threshold:   30.0,
		Duration:    5 * time.Minute,
		Severity:    SeverityWarning,
		Summary:     "Slow tool execution times",
		Description: "95th percentile tool execution time is above {{ .Threshold }}s for {{ .Duration }}",
		Labels:      map[string]string{"service": "mcp-server", "component": "performance"},
		Channels:    []string{"default"},
		Enabled:     true,
	},
	{
		Name:        "ExternalAPIFailures",
		Query:       "external_api_failure_rate",
		Condition:   AlertCondition{Operator: OperatorGreaterThan, Value: 25.0, Duration: 3 * time.Minute},
		Threshold:   25.0,
		Duration:    3 * time.Minute,
		Severity:    SeverityHigh,
		Summary:     "External API failures",
		Description: "External API failure rate is above {{ .Threshold }}% for {{ .Duration }}",
		Labels:      map[string]string{"service": "mcp-server", "component": "external"},
		Channels:    []string{"default"},
		Enabled:     true,
	},
	{
		Name:        "MCPClientDisconnections",
		Query:       "client_disconnection_rate",
		Condition:   AlertCondition{Operator: OperatorGreaterThan, Value: 5.0, Duration: 2 * time.Minute},
		Threshold:   5.0,
		Duration:    2 * time.Minute,
		Severity:    SeverityWarning,
		Summary:     "High MCP client disconnection rate",
		Description: "MCP client disconnection rate is above {{ .Threshold }} per minute for {{ .Duration }}",
		Labels:      map[string]string{"service": "mcp-server", "component": "clients"},
		Channels:    []string{"default"},
		Enabled:     true,
	},
}

func NewAlertManager(config AlertConfig) *AlertManager {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	if config.EvaluationInterval == 0 {
		config.EvaluationInterval = 30 * time.Second
	}
	if config.MaxAlertHistory == 0 {
		config.MaxAlertHistory = 1000
	}
	if config.RateLimitDuration == 0 {
		config.RateLimitDuration = 5 * time.Minute
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 10 * time.Second
	}

	am := &AlertManager{
		config:    config,
		logger:    logger,
		channels:  make(map[string]AlertChannel),
		rules:     DefaultAlertRules,
		alerts:    make(map[string]*Alert),
		history:   make([]Alert, 0),
		stopChan:  make(chan struct{}),
		rateLimit: make(map[string]time.Time),
		silences:  make(map[string]AlertSilence),
	}

	return am
}

func (a *AlertManager) RegisterChannel(channel AlertChannel) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.channels[channel.Name()] = channel
}

func (a *AlertManager) AddRule(rule AlertRule) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.rules = append(a.rules, rule)
}

func (a *AlertManager) Start(metricProvider MetricProvider) {
	if !a.config.EnableAlerting {
		a.logger.Info("Alerting disabled")
		return
	}

	a.ticker = time.NewTicker(a.config.EvaluationInterval)

	go func() {
		for {
			select {
			case <-a.ticker.C:
				a.evaluateRules(metricProvider)
			case <-a.stopChan:
				return
			}
		}
	}()

	a.logger.WithField("interval", a.config.EvaluationInterval).Info("Alert manager started")
}

func (a *AlertManager) Stop() {
	if a.ticker != nil {
		a.ticker.Stop()
	}
	close(a.stopChan)
	a.logger.Info("Alert manager stopped")
}

func (a *AlertManager) evaluateRules(provider MetricProvider) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	for _, rule := range a.rules {
		if !rule.Enabled {
			continue
		}

		value, err := provider.GetMetric(rule.Query)
		if err != nil {
			a.logger.WithError(err).WithField("rule", rule.Name).Error("Failed to get metric for rule")
			continue
		}

		shouldFire := a.evaluateCondition(rule.Condition, value)
		alertID := a.generateAlertID(rule.Name, rule.Labels)

		existingAlert, exists := a.alerts[alertID]

		if shouldFire {
			if !exists {
				// Create new alert
				alert := &Alert{
					ID:          alertID,
					RuleName:    rule.Name,
					Severity:    rule.Severity,
					Status:      StatusFiring,
					Summary:     rule.Summary,
					Description: a.renderTemplate(rule.Description, map[string]interface{}{
						"Threshold": rule.Threshold,
						"Duration":  rule.Duration,
						"Value":     value,
					}),
					Labels:      rule.Labels,
					Annotations: rule.Annotations,
					Value:       value,
					Threshold:   rule.Threshold,
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
					Count:       1,
				}

				a.alerts[alertID] = alert
				a.sendAlert(alert, rule.Channels)
				a.logAlert("fired", alert)

			} else if existingAlert.Status == StatusResolved {
				// Re-fire resolved alert
				existingAlert.Status = StatusFiring
				existingAlert.ResolvedAt = nil
				existingAlert.UpdatedAt = time.Now()
				existingAlert.Count++
				existingAlert.Value = value

				a.sendAlert(existingAlert, rule.Channels)
				a.logAlert("refired", existingAlert)
			} else {
				// Update existing firing alert
				existingAlert.UpdatedAt = time.Now()
				existingAlert.Value = value
				existingAlert.Count++
			}

			// Update rule stats
			rule.LastFired = &existingAlert.UpdatedAt
			rule.FireCount++

		} else if exists && existingAlert.Status == StatusFiring {
			// Resolve alert
			existingAlert.Status = StatusResolved
			existingAlert.UpdatedAt = time.Now()
			now := time.Now()
			existingAlert.ResolvedAt = &now

			a.sendAlert(existingAlert, rule.Channels)
			a.logAlert("resolved", existingAlert)
		}
	}

	a.cleanupOldAlerts()
}

func (a *AlertManager) evaluateCondition(condition AlertCondition, value float64) bool {
	switch condition.Operator {
	case OperatorGreaterThan:
		return value > condition.Value
	case OperatorGreaterEqual:
		return value >= condition.Value
	case OperatorLessThan:
		return value < condition.Value
	case OperatorLessEqual:
		return value <= condition.Value
	case OperatorEqual:
		return value == condition.Value
	case OperatorNotEqual:
		return value != condition.Value
	default:
		return false
	}
}

func (a *AlertManager) sendAlert(alert *Alert, channels []string) {
	if a.isSilenced(alert) {
		alert.Status = StatusSilenced
		return
	}

	if a.isRateLimited(alert.ID) {
		return
	}

	ctx := context.Background()

	for _, channelName := range channels {
		channel, exists := a.channels[channelName]
		if !exists {
			a.logger.WithField("channel", channelName).Warn("Alert channel not found")
			continue
		}

		err := a.sendWithRetry(ctx, channel, alert)
		if err != nil {
			a.logger.WithError(err).WithFields(logrus.Fields{
				"channel": channelName,
				"alert":   alert.ID,
			}).Error("Failed to send alert")
		} else {
			now := time.Now()
			alert.LastSent = &now
			a.rateLimit[alert.ID] = now
		}
	}

	// Add to history
	a.addToHistory(*alert)
}

func (a *AlertManager) sendWithRetry(ctx context.Context, channel AlertChannel, alert *Alert) error {
	var lastError error

	for attempt := 0; attempt <= a.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(a.config.RetryDelay * time.Duration(attempt))
		}

		err := channel.Send(ctx, alert)
		if err == nil {
			return nil
		}

		lastError = err
		a.logger.WithError(err).WithFields(logrus.Fields{
			"channel": channel.Name(),
			"alert":   alert.ID,
			"attempt": attempt + 1,
		}).Warn("Alert send attempt failed")
	}

	return fmt.Errorf("failed to send alert after %d attempts: %w", a.config.RetryAttempts+1, lastError)
}

func (a *AlertManager) isSilenced(alert *Alert) bool {
	for _, silence := range a.silences {
		if time.Now().Before(silence.StartsAt) || time.Now().After(silence.EndsAt) {
			continue
		}

		if a.matchSilence(alert, silence) {
			return true
		}
	}
	return false
}

func (a *AlertManager) matchSilence(alert *Alert, silence AlertSilence) bool {
	for key, value := range silence.Matchers {
		if alertValue, exists := alert.Labels[key]; !exists || alertValue != value {
			return false
		}
	}
	return true
}

func (a *AlertManager) isRateLimited(alertID string) bool {
	if lastSent, exists := a.rateLimit[alertID]; exists {
		return time.Since(lastSent) < a.config.RateLimitDuration
	}
	return false
}

func (a *AlertManager) addToHistory(alert Alert) {
	a.history = append(a.history, alert)

	// Maintain max history size
	if len(a.history) > a.config.MaxAlertHistory {
		a.history = a.history[len(a.history)-a.config.MaxAlertHistory:]
	}
}

func (a *AlertManager) cleanupOldAlerts() {
	cutoff := time.Now().Add(-24 * time.Hour) // Keep resolved alerts for 24 hours

	for id, alert := range a.alerts {
		if alert.Status == StatusResolved && alert.ResolvedAt != nil && alert.ResolvedAt.Before(cutoff) {
			delete(a.alerts, id)
		}
	}

	// Cleanup expired silences
	for id, silence := range a.silences {
		if time.Now().After(silence.EndsAt) {
			delete(a.silences, id)
		}
	}

	// Cleanup rate limits
	for id, lastSent := range a.rateLimit {
		if time.Since(lastSent) > a.config.RateLimitDuration*2 {
			delete(a.rateLimit, id)
		}
	}
}

func (a *AlertManager) generateAlertID(ruleName string, labels map[string]string) string {
	var parts []string
	parts = append(parts, ruleName)
	
	for key, value := range labels {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	
	return strings.Join(parts, ":")
}

func (a *AlertManager) renderTemplate(template string, vars map[string]interface{}) string {
	result := template
	for key, value := range vars {
		placeholder := fmt.Sprintf("{{ .%s }}", key)
		replacement := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, replacement)
	}
	return result
}

func (a *AlertManager) logAlert(action string, alert *Alert) {
	a.logger.WithFields(logrus.Fields{
		"action":     action,
		"alert_id":   alert.ID,
		"rule_name":  alert.RuleName,
		"severity":   alert.Severity,
		"summary":    alert.Summary,
		"value":      alert.Value,
		"threshold":  alert.Threshold,
		"count":      alert.Count,
	}).Info("Alert action")
}

func (a *AlertManager) GetActiveAlerts() []*Alert {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	var active []*Alert
	for _, alert := range a.alerts {
		if alert.Status == StatusFiring {
			active = append(active, alert)
		}
	}
	return active
}

func (a *AlertManager) GetAlertHistory() []Alert {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	history := make([]Alert, len(a.history))
	copy(history, a.history)
	return history
}

func (a *AlertManager) CreateSilence(matchers map[string]string, duration time.Duration, createdBy, comment string) string {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	silence := AlertSilence{
		ID:        fmt.Sprintf("silence_%d", time.Now().UnixNano()),
		Matchers:  matchers,
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(duration),
		CreatedBy: createdBy,
		Comment:   comment,
	}

	a.silences[silence.ID] = silence
	return silence.ID
}

// MetricProvider interface for getting metric values
type MetricProvider interface {
	GetMetric(query string) (float64, error)
}

// Default alert channels
type SlackChannel struct {
	name     string
	webhookURL string
	channel  string
	username string
}

type EmailChannel struct {
	name    string
	to      []string
	from    string
	smtpHost string
	smtpPort int
}

type WebhookChannel struct {
	name string
	url  string
}

func NewSlackChannel(name, webhookURL, channel, username string) *SlackChannel {
	return &SlackChannel{
		name:       name,
		webhookURL: webhookURL,
		channel:    channel,
		username:   username,
	}
}

func (s *SlackChannel) Name() string {
	return s.name
}

func (s *SlackChannel) Send(ctx context.Context, alert *Alert) error {
	color := "good"
	switch alert.Severity {
	case SeverityCritical:
		color = "danger"
	case SeverityHigh:
		color = "danger"
	case SeverityWarning:
		color = "warning"
	}

	payload := map[string]interface{}{
		"channel":   s.channel,
		"username":  s.username,
		"icon_emoji": ":warning:",
		"attachments": []map[string]interface{}{
			{
				"color":     color,
				"title":     alert.Summary,
				"text":      alert.Description,
				"timestamp": alert.CreatedAt.Unix(),
				"fields": []map[string]interface{}{
					{"title": "Severity", "value": string(alert.Severity), "short": true},
					{"title": "Status", "value": string(alert.Status), "short": true},
					{"title": "Value", "value": strconv.FormatFloat(alert.Value, 'f', 2, 64), "short": true},
					{"title": "Count", "value": strconv.Itoa(alert.Count), "short": true},
				},
			},
		},
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", s.webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *SlackChannel) Test(ctx context.Context) error {
	testAlert := &Alert{
		ID:          "test",
		Summary:     "Test Alert",
		Description: "This is a test alert",
		Severity:    SeverityInfo,
		Status:      StatusFiring,
		CreatedAt:   time.Now(),
		Value:       1.0,
		Count:       1,
	}
	return s.Send(ctx, testAlert)
}

func NewWebhookChannel(name, url string) *WebhookChannel {
	return &WebhookChannel{
		name: name,
		url:  url,
	}
}

func (w *WebhookChannel) Name() string {
	return w.name
}

func (w *WebhookChannel) Send(ctx context.Context, alert *Alert) error {
	jsonPayload, _ := json.Marshal(alert)
	req, err := http.NewRequestWithContext(ctx, "POST", w.url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func (w *WebhookChannel) Test(ctx context.Context) error {
	testAlert := &Alert{
		ID:          "test",
		Summary:     "Test Alert",
		Description: "This is a test alert",
		Severity:    SeverityInfo,
		Status:      StatusFiring,
		CreatedAt:   time.Now(),
		Value:       1.0,
		Count:       1,
	}
	return w.Send(ctx, testAlert)
}