package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	graphqlHandler "github.com/temcen/pirex/internal/graphql"
)

// GraphQLHandler handles GraphQL requests
type GraphQLHandler struct {
	graphqlHandler *graphqlHandler.GraphQLHandler
	logger         *logrus.Logger
}

// NewGraphQLHandler creates a new GraphQL handler
func NewGraphQLHandler(
	graphqlHandler *graphqlHandler.GraphQLHandler,
	logger *logrus.Logger,
) *GraphQLHandler {
	return &GraphQLHandler{
		graphqlHandler: graphqlHandler,
		logger:         logger,
	}
}

// GraphQLRequest represents a GraphQL request
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
	OperationName string                 `json:"operationName,omitempty"`
}

// Handle processes GraphQL requests
func (h *GraphQLHandler) Handle(c *gin.Context) {
	var req GraphQLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_GRAPHQL_REQUEST",
				"message": "Invalid GraphQL request format",
			},
		})
		return
	}

	// Validate query complexity (basic implementation)
	if err := h.validateQueryComplexity(req.Query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "QUERY_TOO_COMPLEX",
				"message": err.Error(),
			},
		})
		return
	}

	// For now, return a placeholder response
	// In a full implementation, this would execute the GraphQL query
	result := map[string]interface{}{
		"data": map[string]interface{}{
			"message": "GraphQL endpoint is ready for implementation",
		},
	}

	h.logger.Info("GraphQL query received", "query", req.Query)
	c.JSON(http.StatusOK, result)
}

// HandlePlayground serves the GraphQL playground
func (h *GraphQLHandler) HandlePlayground(c *gin.Context) {
	playgroundHTML := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>GraphQL Playground</title>
    <link rel="stylesheet" href="//cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css" />
    <link rel="shortcut icon" href="//cdn.jsdelivr.net/npm/graphql-playground-react/build/favicon.png" />
    <script src="//cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
</head>
<body>
    <div id="root">
        <style>
            body { margin: 0; font-family: Open Sans, sans-serif; overflow: hidden; }
            #root { height: 100vh; }
        </style>
    </div>
    <script>
        window.addEventListener('load', function (event) {
            GraphQLPlayground.init(document.getElementById('root'), {
                endpoint: '/graphql',
                settings: {
                    'editor.theme': 'light',
                    'editor.fontSize': 14,
                    'editor.fontFamily': '"Source Code Pro", "Consolas", "Inconsolata", "Droid Sans Mono", "Monaco", monospace',
                    'request.credentials': 'include',
                },
                tabs: [
                    {
                        endpoint: '/graphql',
                        query: '# Welcome to GraphQL Playground\n# Type queries in this side of the screen, and you will see intelligent typeaheads aware of the current GraphQL type schema and live syntax and validation errors highlighted within the text.\n\n# Here is an example query:\nquery GetRecommendations($userId: ID!) {\n  recommendations(userId: $userId, filters: { explain: true }) {\n    recommendations {\n      item_id\n      score\n      algorithm\n      explanation\n      confidence\n      position\n    }\n    user_id\n    context\n    generated_at\n    cache_hit\n  }\n}',
                        variables: '{\n  "userId": "123e4567-e89b-12d3-a456-426614174000"\n}',
                    },
                ],
            })
        })
    </script>
</body>
</html>`

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, playgroundHTML)
}

// validateQueryComplexity performs basic query complexity validation
func (h *GraphQLHandler) validateQueryComplexity(query string) error {
	// Simple implementation: count query depth by counting nested braces
	depth := 0
	maxDepth := 0

	for _, char := range query {
		switch char {
		case '{':
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
		case '}':
			depth--
		}
	}

	if maxDepth > 10 {
		return fmt.Errorf("query depth exceeds maximum allowed depth of 10")
	}

	return nil
}
