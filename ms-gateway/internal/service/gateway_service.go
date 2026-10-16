package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/CakeForKit/rsoi-lab2/lb-core/model"
)

type GatewayService struct {
	client                         *http.Client
	flightURL, ticketURL, bonusURL string
}

func New() *GatewayService {
	return &GatewayService{client: &http.Client{Timeout: 5 * time.Second}, flightURL: "http://ms-flight:8060", ticketURL: "http://ms-ticket:8070", bonusURL: "http://ms-bonus:8050"}
}
func (s *GatewayService) call(method, base, path, username string, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			return e
		}
		r = bytes.NewReader(b)
	}
	req, e := http.NewRequest(method, base+path, r)
	if e != nil {
		return e
	}
	req.Header.Set("X-User-Name", username)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := s.client.Do(req)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var x struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&x)
		if x.Message == "" {
			x.Message = resp.Status
		}
		return fmt.Errorf("%s", x.Message)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
func (s *GatewayService) Flights(page, size int) (model.PaginationResponse, error) {
	var r model.PaginationResponse
	return r, s.call(http.MethodGet, s.flightURL, "/internal/flights?page="+fmt.Sprint(page)+"&size="+fmt.Sprint(size), "", nil, &r)
}
func (s *GatewayService) flight(number string) (model.FlightResponse, error) {
	var r model.FlightResponse
	return r, s.call(http.MethodGet, s.flightURL, "/internal/flights/"+url.PathEscape(number), "", nil, &r)
}
func (s *GatewayService) GetBonus(username string) (model.PrivilegeInfoResponse, error) {
	var r model.PrivilegeInfoResponse
	return r, s.call(http.MethodGet, s.bonusURL, "/internal/bonus", username, nil, &r)
}
func (s *GatewayService) tickets(username string) ([]model.Ticket, error) {
	var r []model.Ticket
	return r, s.call(http.MethodGet, s.ticketURL, "/internal/tickets", username, nil, &r)
}
func (s *GatewayService) ticket(username, uid string) (model.Ticket, error) {
	var r model.Ticket
	return r, s.call(http.MethodGet, s.ticketURL, "/internal/tickets/"+url.PathEscape(uid), username, nil, &r)
}
func (s *GatewayService) ticketResponse(t model.Ticket) (model.TicketResponse, error) {
	f, e := s.flight(t.FlightNumber)
	if e != nil {
		return model.TicketResponse{}, e
	}
	return model.TicketResponse{TicketUID: t.TicketUID, FlightNumber: t.FlightNumber, FromAirport: f.FromAirport, ToAirport: f.ToAirport, Date: f.Date, Price: t.Price, Status: t.Status}, nil
}
func (s *GatewayService) Tickets(username string) ([]model.TicketResponse, error) {
	ts, e := s.tickets(username)
	if e != nil {
		return nil, e
	}
	r := make([]model.TicketResponse, 0, len(ts))
	for _, t := range ts {
		x, e := s.ticketResponse(t)
		if e != nil {
			return nil, e
		}
		r = append(r, x)
	}
	return r, nil
}
func (s *GatewayService) Ticket(username, uid string) (model.TicketResponse, error) {
	t, e := s.ticket(username, uid)
	if e != nil {
		return model.TicketResponse{}, e
	}
	return s.ticketResponse(t)
}
func (s *GatewayService) applyBonus(username string, op model.PrivilegeOperationRequest) (model.PrivilegeShortInfo, error) {
	var r model.PrivilegeShortInfo
	return r, s.call(http.MethodPost, s.bonusURL, "/internal/bonus/operations", username, op, &r)
}
func (s *GatewayService) Purchase(username string, req model.TicketPurchaseRequest) (model.TicketPurchaseResponse, error) {
	if username == "" || req.FlightNumber == "" || req.Price <= 0 {
		return model.TicketPurchaseResponse{}, fmt.Errorf("invalid ticket purchase request")
	}
	f, e := s.flight(req.FlightNumber)
	if e != nil {
		return model.TicketPurchaseResponse{}, e
	}
	if req.Price != f.Price {
		return model.TicketPurchaseResponse{}, fmt.Errorf("ticket price does not match flight price")
	}
	var balance model.PrivilegeInfoResponse
	if req.PaidFromBalance {
		balance, e = s.GetBonus(username)
		if e != nil {
			return model.TicketPurchaseResponse{}, e
		}
	}
	var t model.Ticket
	e = s.call(http.MethodPost, s.ticketURL, "/internal/tickets", username, map[string]any{"flightNumber": req.FlightNumber, "price": req.Price}, &t)
	if e != nil {
		return model.TicketPurchaseResponse{}, e
	}
	paidBonuses := 0
	var p model.PrivilegeShortInfo
	if req.PaidFromBalance {
		paidBonuses = min(balance.Balance, req.Price)
		p, e = s.applyBonus(username, model.PrivilegeOperationRequest{TicketUID: t.TicketUID, Amount: paidBonuses, OperationType: model.PrivilegeOperationDebit})
	} else {
		p, e = s.applyBonus(username, model.PrivilegeOperationRequest{TicketUID: t.TicketUID, Amount: req.Price / 10, OperationType: model.PrivilegeOperationFillInBalance})
	}
	if e != nil {
		return model.TicketPurchaseResponse{}, e
	}
	tr, _ := s.ticketResponse(t)
	return model.TicketPurchaseResponse{TicketResponse: tr, PaidByMoney: req.Price - paidBonuses, PaidByBonuses: paidBonuses, Privilege: p}, nil
}
func (s *GatewayService) Cancel(username, uid string) error {
	t, e := s.ticket(username, uid)
	if e != nil {
		return e
	}
	p, e := s.GetBonus(username)
	if e != nil {
		return e
	}
	var related *model.PrivilegeHistory
	for i := range p.History {
		if p.History[i].TicketUID == t.TicketUID {
			related = &p.History[i]
			break
		}
	}
	if e = s.call(http.MethodPut, s.ticketURL, "/internal/tickets/"+url.PathEscape(uid)+"/cancel", username, nil, nil); e != nil {
		return e
	}
	if related != nil {
		operation := model.PrivilegeOperationDebit
		if related.OperationType == model.PrivilegeOperationDebit {
			operation = model.PrivilegeOperationFillInBalance
		}
		_, e = s.applyBonus(username, model.PrivilegeOperationRequest{TicketUID: t.TicketUID, Amount: abs(related.BalanceDiff), OperationType: operation})
	}
	return e
}
func (s *GatewayService) Me(username string) (model.UserInfoResponse, error) {
	tickets, e := s.Tickets(username)
	if e != nil {
		return model.UserInfoResponse{}, e
	}
	p, e := s.GetBonus(username)
	if e != nil {
		return model.UserInfoResponse{}, e
	}
	return model.UserInfoResponse{Tickets: tickets, Privilege: model.PrivilegeShortInfo{Balance: p.Balance, Status: p.Status}}, nil
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
