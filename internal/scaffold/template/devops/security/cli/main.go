package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Constants for report formatting
const countFormatString = "  %s: %d\n"

// Custom time type to handle SonarQube's timestamp format
type SonarTime struct {
	time.Time
}

func (st *SonarTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" {
		st.Time = time.Time{}
		return nil
	}

	// Try multiple time formats that SonarQube might use
	formats := []string{
		"2006-01-02T15:04:05-0700",     // SonarQube format with +0000
		"2006-01-02T15:04:05Z0700",     // Alternative format
		time.RFC3339,                   // Standard RFC3339
		"2006-01-02T15:04:05.000Z",     // With milliseconds
		"2006-01-02T15:04:05.000-0700", // With milliseconds and timezone
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			st.Time = t
			return nil
		}
	}

	return fmt.Errorf("cannot parse time: %s", s)
}

// SonarQube API structures
type SonarIssue struct {
	Key          string    `json:"key"`
	Type         string    `json:"type"`
	Severity     string    `json:"severity"`
	Status       string    `json:"status"`
	Component    string    `json:"component"`
	Line         *int      `json:"line,omitempty"`
	Message      string    `json:"message"`
	Author       string    `json:"author,omitempty"`
	CreationDate SonarTime `json:"creationDate"`
	UpdateDate   SonarTime `json:"updateDate"`
	Rule         string    `json:"rule"`
	Project      string    `json:"project"`
	Tags         []string  `json:"tags,omitempty"`
	Effort       string    `json:"effort,omitempty"`
	Debt         string    `json:"debt,omitempty"`
	TextRange    *struct {
		StartLine   int `json:"startLine"`
		EndLine     int `json:"endLine"`
		StartOffset int `json:"startOffset"`
		EndOffset   int `json:"endOffset"`
	} `json:"textRange,omitempty"`
}

type SonarIssuesResponse struct {
	Issues []SonarIssue `json:"issues"`
	Total  int          `json:"total"`
	Paging struct {
		PageIndex int `json:"pageIndex"`
		PageSize  int `json:"pageSize"`
		Total     int `json:"total"`
	} `json:"paging"`
}

type SonarClient struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func NewSonarClient(baseURL, token string) *SonarClient {
	return &SonarClient{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		Token:   token,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *SonarClient) makeRequest(endpoint string, params url.Values) (*http.Response, error) {
	reqURL := fmt.Sprintf("%s%s", c.BaseURL, endpoint)
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Token, "")
	req.Header.Set("Accept", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	return resp, nil
}

func (c *SonarClient) GetAllIssues(projectKey, orgKey string) ([]SonarIssue, error) {
	var allIssues []SonarIssue
	pageSize := 500
	page := 1

	log.Printf("Querying issues for project: %s", projectKey)

	for {
		params := url.Values{
			"componentKeys": {projectKey},
			"organization":  {orgKey},
			"issueStatuses": {"OPEN", "CONFIRMED", "REOPENED"},
			"ps":            {strconv.Itoa(pageSize)},
			"p":             {strconv.Itoa(page)},
		}

		log.Printf("Fetching page %d...", page)
		resp, err := c.makeRequest("/api/issues/search", params)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var issuesResp SonarIssuesResponse
		if err := json.NewDecoder(resp.Body).Decode(&issuesResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		allIssues = append(allIssues, issuesResp.Issues...)
		log.Printf("Retrieved %d issues from page %d (total so far: %d)", len(issuesResp.Issues), page, len(allIssues))

		if len(issuesResp.Issues) < pageSize || len(allIssues) >= issuesResp.Total {
			break
		}

		page++
	}

	return allIssues, nil
}

func (c *SonarClient) ValidateConnection() error {
	resp, err := c.makeRequest("/api/authentication/validate", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed with status %d", resp.StatusCode)
	}

	return nil
}

func (c *SonarClient) ProjectExists(projectKey, orgKey string) (bool, error) {
	params := url.Values{
		"projects":     {projectKey},
		"organization": {orgKey},
	}

	resp, err := c.makeRequest("/api/projects/search", params)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("project search failed with status %d", resp.StatusCode)
	}

	var result struct {
		Components []struct {
			Key string `json:"key"`
		} `json:"components"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	for _, component := range result.Components {
		if component.Key == projectKey {
			return true, nil
		}
	}

	return false, nil
}

// Report generation functions
func generateJSONReport(issues []SonarIssue, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(issues); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

func generateCSVReport(issues []SonarIssue, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"key", "type", "severity", "status", "component", "line",
		"message", "author", "creationDate", "updateDate", "rule", "project",
	}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data
	for _, issue := range issues {
		line := ""
		if issue.Line != nil {
			line = strconv.Itoa(*issue.Line)
		}

		record := []string{
			issue.Key,
			issue.Type,
			issue.Severity,
			issue.Status,
			issue.Component,
			line,
			issue.Message,
			issue.Author,
			issue.CreationDate.Time.Format(time.RFC3339),
			issue.UpdateDate.Time.Format(time.RFC3339),
			issue.Rule,
			issue.Project,
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}

func generateSummaryReport(issues []SonarIssue, projectKey, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create summary file: %w", err)
	}
	defer file.Close()

	// Count issues by type
	typeCount := make(map[string]int)
	severityCount := make(map[string]int)
	statusCount := make(map[string]int)
	componentCount := make(map[string]int)

	for _, issue := range issues {
		typeCount[issue.Type]++
		severityCount[issue.Severity]++
		statusCount[issue.Status]++
		componentCount[issue.Component]++
	}

	// Write summary
	fmt.Fprintf(file, "SonarQube Issues Summary Report\n")
	fmt.Fprintf(file, "===============================\n")
	fmt.Fprintf(file, "Project: %s\n", projectKey)
	fmt.Fprintf(file, "Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "Total Issues: %d\n\n", len(issues))

	// Issues by type
	fmt.Fprintf(file, "Issues by Type:\n")
	for issueType, count := range typeCount {
		fmt.Fprintf(file, countFormatString, issueType, count)
	}
	fmt.Fprintf(file, "\n")

	// Issues by severity
	fmt.Fprintf(file, "Issues by Severity:\n")
	severityOrder := []string{"BLOCKER", "CRITICAL", "MAJOR", "MINOR", "INFO"}
	for _, severity := range severityOrder {
		if count, exists := severityCount[severity]; exists {
			fmt.Fprintf(file, countFormatString, severity, count)
		}
	}
	fmt.Fprintf(file, "\n")

	// Issues by status
	fmt.Fprintf(file, "Issues by Status:\n")
	for status, count := range statusCount {
		fmt.Fprintf(file, countFormatString, status, count)
	}
	fmt.Fprintf(file, "\n")

	// Top 10 components
	type componentStat struct {
		name  string
		count int
	}
	var components []componentStat
	for comp, count := range componentCount {
		components = append(components, componentStat{comp, count})
	}
	sort.Slice(components, func(i, j int) bool {
		return components[i].count > components[j].count
	})

	fmt.Fprintf(file, "Top 10 Components with Most Issues:\n")
	limit := 10
	if len(components) < limit {
		limit = len(components)
	}
	for i := 0; i < limit; i++ {
		fmt.Fprintf(file, countFormatString, components[i].name, components[i].count)
	}

	return nil
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "sonar-cli",
		Short: "SonarQube CLI tool",
		Long:  `A command-line tool for querying SonarQube issues and generating reports`,
	}

	issuesCmd := &cobra.Command{
		Use:   "issues [sonar-url] [sonar-token] [project-key]",
		Short: "Query all issues for a project",
		Long: `Query SonarQube API for all issues in a project and generate reports.

Examples:
  sonar-cli issues https://sonar.example.com squ_abc123... my-project-key
  sonar-cli issues https://sonar.example.com squ_abc123... my-project-key --output-dir ./reports`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			sonarURL := args[0]
			sonarToken := args[1]
			projectKey := args[2]
			orgKey := os.Getenv("SONAR_ORG_KEY")
			
			if os.Getenv("SONAR_HOST_URL") != "" {
				sonarURL = os.Getenv("SONAR_HOST_URL")
			}

			if os.Getenv("SONAR_TOKEN") != "" {
				sonarToken = os.Getenv("SONAR_TOKEN")
			}

			if os.Getenv("SONAR_PROJECT_KEY") != "" {
				projectKey = os.Getenv("SONAR_PROJECT_KEY")
			}

			outputDir, _ := cmd.Flags().GetString("output-dir")
			if outputDir == "" {
				outputDir = "./sonar-reports"
			}

			return executeQuery(sonarURL, sonarToken, projectKey, orgKey, outputDir)
		},
	}

	issuesCmd.Flags().StringP("output-dir", "o", "", "Output directory for reports (default: ./sonar-reports)")
	rootCmd.AddCommand(issuesCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func executeQuery(sonarURL, sonarToken, projectKey, orgKey, outputDir string) error {
	log.Printf("Starting SonarQube issues query for project: %s", projectKey)
	log.Printf("SonarQube URL: %s", sonarURL)

	// Create SonarQube client
	client := NewSonarClient(sonarURL, sonarToken)

	// Validate connection
	log.Printf("Testing SonarQube connection and authentication...")
	if err := client.ValidateConnection(); err != nil {
		return fmt.Errorf("failed to authenticate with SonarQube: %w", err)
	}
	log.Printf("Authentication successful")

	// Check if project exists
	log.Printf("Verifying project exists...")
	exists, err := client.ProjectExists(projectKey, orgKey)
	if err != nil {
		return fmt.Errorf("failed to verify project existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("project '%s' not found in SonarQube", projectKey)
	}
	log.Printf("Project found")

	// Query all issues
	log.Printf("Querying all issues for project: %s", projectKey)
	issues, err := client.GetAllIssues(projectKey, orgKey)
	if err != nil {
		return fmt.Errorf("failed to query issues: %w", err)
	}
	log.Printf("Retrieved %d total issues", len(issues))

	// Create output directory
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate timestamp for report files
	timestamp := time.Now().Format("20060102_150405")

	// Generate reports
	jsonFile := filepath.Join(outputDir, fmt.Sprintf("sonar-issues-%s-%s.json", projectKey, timestamp))
	csvFile := filepath.Join(outputDir, fmt.Sprintf("sonar-issues-%s-%s.csv", projectKey, timestamp))
	summaryFile := filepath.Join(outputDir, fmt.Sprintf("sonar-summary-%s-%s.txt", projectKey, timestamp))

	log.Printf("Generating JSON report...")
	if err := generateJSONReport(issues, jsonFile); err != nil {
		return fmt.Errorf("failed to generate JSON report: %w", err)
	}
	log.Printf("JSON report saved to: %s", jsonFile)

	log.Printf("Generating CSV report...")
	if err := generateCSVReport(issues, csvFile); err != nil {
		return fmt.Errorf("failed to generate CSV report: %w", err)
	}
	log.Printf("CSV report saved to: %s", csvFile)

	log.Printf("Generating summary report...")
	if err := generateSummaryReport(issues, projectKey, summaryFile); err != nil {
		return fmt.Errorf("failed to generate summary report: %w", err)
	}
	log.Printf("Summary report saved to: %s", summaryFile)

	// Display summary to console
	log.Printf("\nIssue Summary:")

	// Count issues by type and severity for console output
	typeCount := make(map[string]int)
	severityCount := make(map[string]int)

	for _, issue := range issues {
		typeCount[issue.Type]++
		severityCount[issue.Severity]++
	}

	log.Printf("Total Issues: %d", len(issues))
	log.Printf("By Type:")
	for issueType, count := range typeCount {
		log.Printf("  %s: %d", issueType, count)
	}
	log.Printf("By Severity:")
	severityOrder := []string{"BLOCKER", "CRITICAL", "MAJOR", "MINOR", "INFO"}
	for _, severity := range severityOrder {
		if count, exists := severityCount[severity]; exists {
			log.Printf("  %s: %d", severity, count)
		}
	}

	log.Printf("\nSonarQube issues query completed successfully!")
	log.Printf("Reports generated:")
	log.Printf("  - JSON (full data): %s", jsonFile)
	log.Printf("  - CSV (summary): %s", csvFile)
	log.Printf("  - TXT (summary): %s", summaryFile)

	return nil
}
