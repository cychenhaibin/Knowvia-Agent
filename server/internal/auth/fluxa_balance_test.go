package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestFluxABalanceFetcherRequestsStatusAndSelf(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			if got := r.Header.Get("Authorization"); got != "" {
				t.Fatalf("status authorization = %q, want no authorization", got)
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000,"quota_display_type":"CNY","usd_exchange_rate":7.2,"custom_currency_symbol":"","custom_currency_exchange_rate":1}}`))
		case "/api/user/self":
			if got := r.Header.Get("Authorization"); got != "Bearer upstream-token" {
				t.Fatalf("self authorization = %q, want upstream bearer token", got)
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota":7400000}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	fetcher := newFluxABalanceFetcher(server.URL, server.URL, server.Client())
	got, err := fetcher.Balance(context.Background(), FluxASitePaid, "upstream-token")
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	want := FluxABalance{
		Quota:                      7_400_000,
		QuotaPerUnit:               500_000,
		DisplayType:                "CNY",
		USDExchangeRate:            7.2,
		CustomCurrencySymbol:       "",
		CustomCurrencyExchangeRate: 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("balance = %#v, want %#v", got, want)
	}
}

func TestFluxABalanceFetcherRejectsUnsuccessfulEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/status" {
			_, _ = w.Write([]byte(`{"success":false,"data":{}}`))
			return
		}
		t.Fatal("user quota endpoint should not be requested after an unsuccessful status envelope")
	}))
	t.Cleanup(server.Close)

	_, err := newFluxABalanceFetcher(server.URL, server.URL, server.Client()).Balance(context.Background(), FluxASitePaid, "upstream-token")
	if !errors.Is(err, ErrFluxAUnavailable) {
		t.Fatalf("balance error = %v, want ErrFluxAUnavailable", err)
	}
}

func TestFluxABalanceFetcherRejectsZeroQuotaPerUnit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":0,"quota_display_type":"USD","usd_exchange_rate":1,"custom_currency_symbol":"","custom_currency_exchange_rate":1}}`))
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota":1}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	_, err := newFluxABalanceFetcher(server.URL, server.URL, server.Client()).Balance(context.Background(), FluxASitePaid, "upstream-token")
	if !errors.Is(err, ErrFluxAUnavailable) {
		t.Fatalf("balance error = %v, want ErrFluxAUnavailable", err)
	}
}

func TestFluxABalanceFetcherMapsUnauthorizedUpstreamResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	_, err := newFluxABalanceFetcher(server.URL, server.URL, server.Client()).Balance(context.Background(), FluxASitePaid, "upstream-token")
	if !errors.Is(err, ErrFluxAReauthenticationRequired) {
		t.Fatalf("balance error = %v, want ErrFluxAReauthenticationRequired", err)
	}
}
