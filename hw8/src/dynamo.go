package main

import (
	"context"
	"errors"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
)

// DynamoDB store (STEP II). Same routes, validation and JSON as the MySQL store (carts.go).
var (
	ddb            *dynamodb.Client
	cartTable      string
	consistentRead bool
)

const cartTTL = 7 * 24 * time.Hour

// openDynamo reads DYNAMODB_TABLE (required), DYNAMODB_CONSISTENT_READ (optional, "true" = strong reads)
// and DYNAMODB_ENDPOINT (optional, for DynamoDB Local). Credentials come from the ECS task role (LabRole).
func openDynamo() {
	cartTable = os.Getenv("DYNAMODB_TABLE")
	if cartTable == "" {
		log.Fatal("missing environment variable DYNAMODB_TABLE")
	}
	consistentRead = os.Getenv("DYNAMODB_CONSISTENT_READ") == "true"
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("aws config: %v", err)
	}
	ddb = dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		if ep := os.Getenv("DYNAMODB_ENDPOINT"); ep != "" {
			o.BaseEndpoint = aws.String(ep)
		}
	})
	log.Printf("cart store: dynamodb table=%s consistentRead=%v", cartTable, consistentRead)
}

func num(v int64) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(v, 10)}
}

func cartKey(id int32) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"cart_id": num(int64(id))}
}

func isConditionFailed(err error) bool {
	var ccf *types.ConditionalCheckFailedException
	return errors.As(err, &ccf)
}

// dynamoCreateCart: POST /shopping-carts. Random int32 ID; the condition rejects an existing ID, so retry.
func dynamoCreateCart(c *gin.Context) {
	var req createCartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_INPUT", "The provided input is invalid")
		return
	}
	now := time.Now().UTC()
	for attempt := 0; attempt < 3; attempt++ {
		id := rand.Int32N(math.MaxInt32) + 1 // [1, 2^31-1]
		_, err := ddb.PutItem(c.Request.Context(), &dynamodb.PutItemInput{
			TableName: aws.String(cartTable),
			Item: map[string]types.AttributeValue{
				"cart_id":     num(int64(id)),
				"customer_id": num(int64(req.CustomerID)),
				"items":       &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
				"created_at":  &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
				"expires_at":  num(now.Add(cartTTL).Unix()),
			},
			ConditionExpression: aws.String("attribute_not_exists(cart_id)"),
		})
		if isConditionFailed(err) {
			continue // ID already taken: try another
		}
		if err != nil {
			respondInternalError(c, "create cart", err)
			return
		}
		c.IndentedJSON(http.StatusCreated, cart{ShoppingCartID: id, CustomerID: req.CustomerID, Items: []cartItem{}})
		return
	}
	respondInternalError(c, "create cart", errors.New("3 cart_id collisions in a row"))
}

// dynamoGetCart: GET /shopping-carts/{id}. One GetItem returns the cart and all its items.
func dynamoGetCart(c *gin.Context) {
	id, ok := parseCartID(c)
	if !ok {
		return
	}
	out, err := ddb.GetItem(c.Request.Context(), &dynamodb.GetItemInput{
		TableName:      aws.String(cartTable),
		Key:            cartKey(id),
		ConsistentRead: aws.Bool(consistentRead),
	})
	if err != nil {
		respondInternalError(c, "get cart", err)
		return
	}
	if out.Item == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "Shopping cart not found")
		return
	}
	result := cart{ShoppingCartID: id, Items: []cartItem{}}
	if v, ok := out.Item["customer_id"].(*types.AttributeValueMemberN); ok {
		n, _ := strconv.ParseInt(v.Value, 10, 32)
		result.CustomerID = int32(n)
	}
	if m, ok := out.Item["items"].(*types.AttributeValueMemberM); ok {
		for pid, qv := range m.Value {
			p, err1 := strconv.ParseInt(pid, 10, 32)
			qn, ok2 := qv.(*types.AttributeValueMemberN)
			if err1 != nil || !ok2 {
				continue
			}
			q, err2 := strconv.ParseInt(qn.Value, 10, 32)
			if err2 != nil {
				continue
			}
			result.Items = append(result.Items, cartItem{ProductID: int32(p), Quantity: int32(q)})
		}
		sort.Slice(result.Items, func(a, b int) bool { return result.Items[a].ProductID < result.Items[b].ProductID })
	}
	c.IndentedJSON(http.StatusOK, result)
}

// dynamoAddItem: POST /shopping-carts/{id}/items. One atomic conditional UpdateItem:
// the cart must exist (else 404), and the quantity is added on the server (no lost updates).
// "items" is a reserved word, so it is always #i.
func dynamoAddItem(c *gin.Context) {
	id, ok := parseCartID(c)
	if !ok {
		return
	}
	var item cartItem
	if err := c.ShouldBindJSON(&item); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_INPUT", "The provided input is invalid")
		return
	}
	_, err := ddb.UpdateItem(c.Request.Context(), &dynamodb.UpdateItemInput{
		TableName:           aws.String(cartTable),
		Key:                 cartKey(id),
		UpdateExpression:    aws.String("SET #i.#p = if_not_exists(#i.#p, :zero) + :q, expires_at = :exp"),
		ConditionExpression: aws.String("attribute_exists(cart_id)"),
		ExpressionAttributeNames: map[string]string{
			"#i": "items",
			"#p": strconv.Itoa(int(item.ProductID)),
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero": num(0),
			":q":    num(int64(item.Quantity)),
			":exp":  num(time.Now().Add(cartTTL).Unix()),
		},
	})
	if isConditionFailed(err) {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "Shopping cart not found")
		return
	}
	if err != nil {
		respondInternalError(c, "add item", err)
		return
	}
	c.Status(http.StatusNoContent)
}

// useDynamo: CART_STORE=dynamodb selects STEP II; anything else (or unset) keeps MySQL (STEP I).
func useDynamo() bool { return os.Getenv("CART_STORE") == "dynamodb" }
