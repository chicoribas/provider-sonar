package sonar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var ErrProjectNotFound = errors.New("Project not found")

type Project struct {
	Organization string `json:"organization"`
	Key          string `json:"key"`
	Name         string `json:"name"`
	Qualifier    string `json:"qualifier"`
	Visibility   string `json:"visibility"`
	// TODO: Custom Unmarshal for Time format: 2022-11-10T19:33:53+0100
	// https://eli.thegreenplace.net/2020/unmarshaling-time-values-from-json/
	LastAnalysisDate string `json:"lastAnalysisDate,omitempty"`
	Revision         string `json:"revision"`
}

type ProjectPage struct {
	Paging   SonarPaging `json:"paging"`
	Projects []Project   `json:"components"`
}

type ProjectClient struct {
	sonarApi SonarApi
}

// Creates a new Project Client
func NewProjectClient(options SonarApiOptions) ProjectClient {
	return ProjectClient{
		sonarApi: NewSonarApi(options),
	}
}

type SearchOptions struct {
	// List of project keys
	Projects []string
	// 1-based page number
	Page int
	// Page size. Must be greater than 0 and less or equal than 500
	PageSize int
}

// Create new project
// https://sonarcloud.io/web_api/api/projects/create
func (projectClient ProjectClient) Create(ctx context.Context, organization string, name string, project string, visibility string) (Project, error) {

	url := projectClient.sonarApi.GetUrl("/api/projects/create")
	params := url.Query()
	params.Add("organization", organization)
	params.Add("name", name)
	params.Add("project", project)
	params.Add("visibility", visibility)

	url.RawQuery = params.Encode()
	client := &http.Client{}

	req, err := projectClient.sonarApi.NewRequest(ctx, "POST", url.String(), nil)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { err = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return Project{}, fmt.Errorf("error calling sonar api: %s", resp.Status)
	}

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return Project{}, err
	}

	var response map[string]Project
	e := json.Unmarshal(responseData, &response)

	return response["project"], e
}

// Delete project
// https://sonarcloud.io/web_api/api/projects/delete
func (projectClient ProjectClient) Delete(ctx context.Context, project string) error {

	url := projectClient.sonarApi.GetUrl("/api/projects/delete")
	params := url.Query()
	params.Add("project", project)
	url.RawQuery = params.Encode()

	client := &http.Client{}
	req, _ := projectClient.sonarApi.NewRequest(ctx, "POST", url.String(), nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { err = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("error calling sonar api: %s", resp.Status)
	}

	return nil

}

// Search calls the "/api/projects/search" endpoint
// https://sonarcloud.io/web_api/api/projects/search
func (projectClient ProjectClient) Search(ctx context.Context, organization string, options SearchOptions) (ProjectPage, error) {

	url := projectClient.sonarApi.GetUrl("/api/projects/search")
	params := url.Query()
	params.Add("organization", organization)

	if len(options.Projects) > 0 {
		params.Add("projects", strings.Join(options.Projects, ","))
	}
	if options.Page > 0 {
		params.Add("p", strconv.Itoa(options.Page))
	}
	if options.PageSize > 0 {
		params.Add("ps", strconv.Itoa(options.PageSize))
	}

	url.RawQuery = params.Encode()

	client := &http.Client{}
	req, err := projectClient.sonarApi.NewRequest(ctx, "GET", url.String(), nil)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { err = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return ProjectPage{}, fmt.Errorf("error calling sonar api: %s", resp.Status)
	}
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return ProjectPage{}, err
	}

	var page ProjectPage
	e := json.Unmarshal(responseData, &page)
	if e != nil {
		return ProjectPage{}, err
	}

	return page, e
}

// Get a single sonar project by project key
func (projectClient ProjectClient) GetByProjectKey(ctx context.Context, organization string, project string) (Project, error) {

	projectPage, err := projectClient.Search(ctx, organization, SearchOptions{Projects: []string{project}})

	if err != nil {
		return Project{}, err
	}

	if len(projectPage.Projects) == 0 {
		return Project{}, ErrProjectNotFound
	}

	return projectPage.Projects[0], nil

}

// Update project visibility
func (projectClient ProjectClient) UpdateVisibility(ctx context.Context, project string, visibility string) error {

	url := projectClient.sonarApi.GetUrl("/api/projects/update_visibility")
	params := url.Query()
	params.Add("project", project)
	params.Add("visibility", visibility)
	url.RawQuery = params.Encode()

	client := &http.Client{}
	req, _ := projectClient.sonarApi.NewRequest(ctx, "POST", url.String(), nil)
	resp, _ := client.Do(req)
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("error calling sonar api: %s", resp.Status)
	}

	return nil

}
