package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"

	"github.com/3ssalunke/crypto-exchange/orderbook"
	"github.com/3ssalunke/crypto-exchange/util"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/labstack/echo/v4"
)

type (
	OrderType string
	Market    string

	Exchange struct {
		client     *ethclient.Client
		users      map[int64]*User
		privateKey *ecdsa.PrivateKey
		orderbooks map[Market]*orderbook.Orderbook
	}

	PlaceOrderRequest struct {
		UserID int64
		Type   OrderType
		Bid    bool
		Size   float64
		Price  float64
		Market Market
	}

	Order struct {
		UserID    int64
		ID        int64
		Price     float64
		Size      float64
		Bid       bool
		Timestamp int64
	}

	OrderbookData struct {
		TotalBidVolume float64
		TotalAskVolume float64
		Asks           []*Order
		Bids           []*Order
	}

	MatchedOrder struct {
		Price float64
		Size  float64
		ID    int64
	}

	User struct {
		ID         int64
		PrivateKey *ecdsa.PrivateKey
	}
)

const (
	MarketOrder OrderType = "MARKET"
	LimitOrder  OrderType = "LIMIT"

	MarketEth Market = "ETH"

	exchangePrivateKey = "6e619f79db14302bf2530cbb83780cea5c5da496ff26f10c8b55bdcc2bc634fe"
)

func StartServer() {
	e := echo.New()
	e.HTTPErrorHandler = httpErrorHandler

	client, err := ethclient.Dial("http://localhost:7545")
	if err != nil {
		log.Fatal(err)
	}

	ex, err := NewExchange(exchangePrivateKey, client)
	if err != nil {
		log.Fatal(err)
	}

	user1 := NewUser(100001, "0b03206a60ee8b9479d86996c5f6616b7232064c8fb6ba976f5064efe3220575")
	ex.users[user1.ID] = user1

	user2 := NewUser(100002, "b668c8f5c88100af522491adc5b34d810925722446d546d45265e6480c810fe1")
	ex.users[user2.ID] = user2

	user3 := NewUser(100003, "ad17226dac8a40c5c5b5a0977f32f3c69b2d6a749608c475a6caaf6e1d25b3e3")
	ex.users[user3.ID] = user3

	e.GET("/book/:market", ex.handleGetBook)
	e.POST("/order", ex.handlePlaceOrder)
	e.DELETE("/order/:orderId", ex.cancelOrder)

	e.Start(":3000")
}

func httpErrorHandler(err error, c echo.Context) {
	fmt.Println(err)
}

func NewUser(id int64, key string) *User {
	privateKey, err := crypto.HexToECDSA(key)
	if err != nil {
		panic(err)
	}

	return &User{
		ID:         id,
		PrivateKey: privateKey,
	}
}

func NewExchange(key string, client *ethclient.Client) (*Exchange, error) {
	orderbooks := make(map[Market]*orderbook.Orderbook)
	orderbooks[MarketEth] = orderbook.NewOrderbook()

	privateKey, err := crypto.HexToECDSA(exchangePrivateKey)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return &Exchange{
		client:     client,
		users:      make(map[int64]*User),
		privateKey: privateKey,
		orderbooks: orderbooks,
	}, nil
}

func (ex *Exchange) handleGetBook(c echo.Context) error {
	market := Market(c.Param("market"))
	ob, ok := ex.orderbooks[market]

	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]any{"msg": "market not found"})
	}

	orderbookData := OrderbookData{
		TotalBidVolume: ob.BidTotalVolume(),
		TotalAskVolume: ob.AskTotalVolume(),
		Asks:           []*Order{},
		Bids:           []*Order{},
	}

	for _, limit := range ob.Asks() {
		for _, order := range limit.Orders {
			o := &Order{
				UserID:    order.UserID,
				ID:        order.ID,
				Price:     limit.Price,
				Size:      order.Size,
				Bid:       order.Bid,
				Timestamp: order.Timestamp,
			}

			orderbookData.Asks = append(orderbookData.Asks, o)
		}
	}

	for _, limit := range ob.Bids() {
		for _, order := range limit.Orders {
			o := &Order{
				UserID:    order.UserID,
				ID:        order.ID,
				Price:     limit.Price,
				Size:      order.Size,
				Bid:       order.Bid,
				Timestamp: order.Timestamp,
			}

			orderbookData.Bids = append(orderbookData.Bids, o)
		}
	}

	return c.JSON(http.StatusOK, orderbookData)
}

func (ex *Exchange) handlePlaceMarketOrder(market Market, o *orderbook.Order) ([]orderbook.Match, []*MatchedOrder) {
	ob := ex.orderbooks[market]
	matches := ob.PlaceMarketOrder(o)
	matchedOrders := make([]*MatchedOrder, len(matches))

	isBid := false
	if o.Bid {
		isBid = true
	}

	for i := 0; i < len(matches); i++ {
		id := matches[i].Bid.ID
		if isBid {
			id = matches[i].Ask.ID
		}
		matchedOrders[i] = &MatchedOrder{
			Size:  matches[i].SizeFilled,
			Price: matches[i].Price,
			ID:    id,
		}
	}

	return matches, matchedOrders
}

func (ex *Exchange) handlePlaceLimitOrder(market Market, price float64, order *orderbook.Order) error {
	ob := ex.orderbooks[market]
	ob.PlaceLimitOrder(price, order)

	return nil
}

func (ex *Exchange) handlePlaceOrder(c echo.Context) error {
	var placeOrderData PlaceOrderRequest

	if err := json.NewDecoder(c.Request().Body).Decode(&placeOrderData); err != nil {
		fmt.Println(err)
		return err
	}

	market := Market(placeOrderData.Market)
	order := orderbook.NewOrder(placeOrderData.Bid, placeOrderData.Size, placeOrderData.UserID)

	if placeOrderData.Type == LimitOrder {
		if err := ex.handlePlaceLimitOrder(market, placeOrderData.Price, order); err != nil {
			return err
		}
		return c.JSON(200, map[string]any{"msg": "limit order placed"})
	}

	if placeOrderData.Type == MarketOrder {
		matches, matchedOrders := ex.handlePlaceMarketOrder(market, order)

		if err := ex.handleMatches(matches); err != nil {
			return err
		}

		return c.JSON(200, map[string]any{"msg": "market order placed", "mathces_found": matchedOrders})
	}

	return nil
}

func (ex *Exchange) handleMatches(matches []orderbook.Match) error {
	for _, match := range matches {
		fromUser, ok := ex.users[match.Ask.UserID]
		if !ok {
			return fmt.Errorf("user not found: %d", match.Ask.UserID)
		}

		toUser, ok := ex.users[match.Bid.UserID]
		if !ok {
			return fmt.Errorf("user not found: %d", match.Ask.UserID)
		}
		toAddress := crypto.PubkeyToAddress(toUser.PrivateKey.PublicKey)

		amount := big.NewInt(int64(match.SizeFilled))

		err := util.TransferEth(ex.client, fromUser.PrivateKey, toAddress, amount)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ex *Exchange) cancelOrder(c echo.Context) error {
	strOrderID := c.Param("orderId")

	orderID, err := strconv.Atoi(strOrderID)
	if err != nil {
		return err
	}

	ob := ex.orderbooks[MarketEth]
	order := ob.Orders[int64(orderID)]

	ob.CancelOrder(order)

	return c.JSON(200, map[string]any{"msg": "order deleted successfully", "order_id": orderID})
}
