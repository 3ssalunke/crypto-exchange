package orderbook

import (
	"fmt"
	"reflect"
	"testing"
)

func assert(t *testing.T, a, b any) {
	if !reflect.DeepEqual(a, b) {
		t.Errorf("%+v != %+v", a, b)
	}
}

func TestLimit(t *testing.T) {
	l := NewLimit(10_000)
	buyOrderA := NewOrder(true, 5, 0)
	buyOrderB := NewOrder(true, 8, 0)
	buyOrderC := NewOrder(true, 3, 0)

	l.AddOrder(buyOrderA)
	l.AddOrder(buyOrderB)
	l.AddOrder(buyOrderC)

	l.DeleteOrder(buyOrderB)

	fmt.Println(l)
}

func TestPlaceLimitOrder(t *testing.T) {
	ob := NewOrderbook()

	sellOrder := NewOrder(false, 10, 0)
	sellOrder1 := NewOrder(false, 10, 0)
	sellOrder2 := NewOrder(false, 10, 0)

	ob.PlaceLimitOrder(18_0000, sellOrder)
	ob.PlaceLimitOrder(10_000, sellOrder1)
	ob.PlaceLimitOrder(12_000, sellOrder2)

	assert(t, len(ob.asks), 1)
}

func TestPlaceMarketOrder(t *testing.T) {
	ob := NewOrderbook()

	sellOrder := NewOrder(false, 20, 0)
	ob.PlaceLimitOrder(18_000, sellOrder)

	buyOrder := NewOrder(true, 10, 0)
	matches := ob.PlaceMarketOrder(buyOrder)

	assert(t, len(matches), 1)
	assert(t, len(ob.asks), 1)
	assert(t, ob.AskTotalVolume(), 10.0)
	assert(t, matches[0].Ask, sellOrder)
	assert(t, matches[0].Bid, buyOrder)
	assert(t, matches[0].SizeFilled, 10.0)
	assert(t, matches[0].Price, 18_000.0)
	assert(t, buyOrder.IsFilled(), true)

	fmt.Printf("%+v", matches)
}

func TestPlaceMarketOrderMultiFill(t *testing.T) {
	ob := NewOrderbook()

	buyOrderA := NewOrder(true, 5, 0)
	buyOrderB := NewOrder(true, 8, 0)
	buyOrderC := NewOrder(true, 8, 0)
	buyOrderD := NewOrder(true, 2, 0)
	buyOrderE := NewOrder(true, 3, 0)

	ob.PlaceLimitOrder(10_000, buyOrderA)
	ob.PlaceLimitOrder(5_000, buyOrderB)
	ob.PlaceLimitOrder(8_000, buyOrderC)
	ob.PlaceLimitOrder(8_000, buyOrderD)
	ob.PlaceLimitOrder(10_000, buyOrderE)

	assert(t, ob.BidTotalVolume(), 44.0)

	sellOrder := NewOrder(false, 20, 0)
	matches := ob.PlaceMarketOrder(sellOrder)

	assert(t, ob.BidTotalVolume(), 6.0)
	assert(t, len(matches), 5)
	assert(t, len(ob.bids), 1)

	fmt.Printf("%+v", matches)

}
