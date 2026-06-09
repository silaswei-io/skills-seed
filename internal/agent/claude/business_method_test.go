package claude

import (
	"encoding/json"
	"testing"
)

func TestParseBatchLearnResultWithBusinessMethod(t *testing.T) {
	jsonStr := `{
  "patterns": [
    {
      "id": "uuid-generator",
      "name": "UUID 生成器",
      "category": "utils",
      "description": "使用 Google UUID 库生成符合 RFC 4122 的 UUID v4",
      "good_example": "",
      "bad_example": "",
      "rule": "当需要生成唯一标识符时，使用 uuid.GenerateUUID()",
      "confidence": 0.95,
      "frequency": 5,
      "business_method": {
        "name": "GenerateUUID",
        "code_location": {"current_location": "internal/utils/uuid.go:15"},
        "description": "生成符合 RFC 4122 的 UUID v4",
        "usage": "在创建新实体、生成追踪ID、或需要唯一标识符时使用",
        "type": "common",
        "function": "返回一个小写的 UUID 字符串，基于随机数生成"
      }
    },
    {
      "id": "rpc-client-wrapper",
      "name": "RPC 客户端封装",
      "category": "utils",
      "description": "封装外部 RPC 调用，包含重试、超时、错误处理",
      "good_example": "",
      "bad_example": "",
      "rule": "调用外部 RPC 服务时，使用统一的 RPC 客户端封装",
      "confidence": 0.90,
      "frequency": 3,
      "business_method": {
        "name": "CallUserRPC",
        "code_location": {"current_location": "internal/infra/rpc/user.go:25"},
        "description": "调用用户服务的 RPC 方法",
        "usage": "需要查询用户信息、验证用户权限时使用",
        "type": "domain",
        "function": "发送 RPC 请求到用户服务，自动处理重试和超时"
      }
    }
  ]
}`

	var result struct {
		Patterns []struct {
			ID             string  `json:"id"`
			Name           string  `json:"name"`
			Category       string  `json:"category"`
			Description    string  `json:"description"`
			GoodExample    string  `json:"good_example"`
			BadExample     string  `json:"bad_example"`
			Rule           string  `json:"rule"`
			Confidence     float64 `json:"confidence"`
			Frequency      int     `json:"frequency"`
			BusinessMethod *struct {
				Name         string `json:"name"`
				CodeLocation struct {
					CurrentLocation string `json:"current_location"`
				} `json:"code_location"`
				Description string `json:"description"`
				Usage       string `json:"usage"`
				Type        string `json:"type"`
				Function    string `json:"function"`
			} `json:"business_method"`
		} `json:"patterns"`
	}

	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(result.Patterns) != 2 {
		t.Fatalf("Expected 2 patterns, got %d", len(result.Patterns))
	}

	// 验证第一个模式（UUID生成器）
	p1 := result.Patterns[0]
	if p1.ID != "uuid-generator" {
		t.Errorf("Expected ID 'uuid-generator', got '%s'", p1.ID)
	}
	if p1.BusinessMethod == nil {
		t.Fatal("Expected BusinessMethod to be non-nil")
	}
	if p1.BusinessMethod.Name != "GenerateUUID" {
		t.Errorf("Expected BusinessMethod.Name 'GenerateUUID', got '%s'", p1.BusinessMethod.Name)
	}
	if p1.BusinessMethod.CodeLocation.CurrentLocation != "internal/utils/uuid.go:15" {
		t.Errorf("Expected CurrentLocation 'internal/utils/uuid.go:15', got '%s'", p1.BusinessMethod.CodeLocation.CurrentLocation)
	}
	if p1.BusinessMethod.Type != "common" {
		t.Errorf("Expected Type 'common', got '%s'", p1.BusinessMethod.Type)
	}

	// 验证第二个模式（RPC封装）
	p2 := result.Patterns[1]
	if p2.ID != "rpc-client-wrapper" {
		t.Errorf("Expected ID 'rpc-client-wrapper', got '%s'", p2.ID)
	}
	if p2.BusinessMethod == nil {
		t.Fatal("Expected BusinessMethod to be non-nil")
	}
	if p2.BusinessMethod.Name != "CallUserRPC" {
		t.Errorf("Expected BusinessMethod.Name 'CallUserRPC', got '%s'", p2.BusinessMethod.Name)
	}
	if p2.BusinessMethod.Type != "domain" {
		t.Errorf("Expected Type 'domain', got '%s'", p2.BusinessMethod.Type)
	}

	t.Logf("✅ Successfully parsed %d patterns with business methods", len(result.Patterns))
}
