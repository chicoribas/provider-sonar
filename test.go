package main

import (
	"context"
	"fmt"

	"github.com/crossplane/provider-sonar/internal/clients/sonar"
)

func main() {

	sonarOptions := sonar.SonarApiOptions{
		Key: "1266f67a3d10669f9b16eaca8caa4cbb9da7de41",
	}
	projectClient := sonar.NewProjectClient(sonarOptions)

	// fmt.Println((projectClient.Search("gbsandbox",
	// 	sonar.SearchOptions{
	// 		// Projects: []string{
	// 		// 	"a2a9f5b0-d60a-4b90-ac63-71043dfd420e",
	// 		// },
	// 		// Page:     3,
	// 		// PageSize: 2,
	// 	},
	// )))

	// fmt.Println((projectClient.GetByProjectKey("chicoribas", "chicoribas_scafflater")))

	fmt.Println((projectClient.Create(context.Background(), "chicoribas", "test_provider_name", "test_provider_key", "public")))
	fmt.Println((projectClient.Delete(context.Background(), "test_provider_key")))
	// fmt.Println((projectClient.UpdateVisibility("test_provider_key", "private")))

}
