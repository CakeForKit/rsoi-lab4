package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/CakeForKit/rsoi-lab2/lb-core/model"
)

const (
	queueRetryTimeout = 10 * time.Second
)

// UnavailableError lets HTTP handlers distinguish a temporary dependency
// outage from a regular request-validation error.
type UnavailableError struct{ Service string }

func (e *UnavailableError) Error() string { return e.Service + " Service unavailable" }

type remoteError struct {
	status  int
	message string
}

func (e *remoteError) Error() string { return e.message }

type cancelJob struct {
	username string
	ticket   model.Ticket
}

type GatewayService struct {
	client                         *http.Client
	flightURL, ticketURL, bonusURL string
	breakers                       map[string]*circuitBreaker
	cancelQueue                    chan cancelJob
}

func New() *GatewayService {
	s := &GatewayService{
		client:    &http.Client{Timeout: 5 * time.Second},
		flightURL: "http://ms-flight:8060",
		ticketURL: "http://ms-ticket:8070",
		bonusURL:  "http://ms-bonus:8050",
		breakers: map[string]*circuitBreaker{
			"Flight": {}, "Ticket": {}, "Bonus": {},
		},
		cancelQueue: make(chan cancelJob, 100),
	}
	go s.processCancellationQueue()
	return s
}

func (s *GatewayService) serviceName(base string) string {
	switch base {
	case s.flightURL:
		return "Flight"
	case s.ticketURL:
		return "Ticket"
	default:
		return "Bonus"
	}
}

func (s *GatewayService) call(method, base, path, username string, body any, out any) error {
	if method == http.MethodGet {
		breaker := s.breakers[s.serviceName(base)]
		if !breaker.allow() {
			return &UnavailableError{Service: s.serviceName(base)}
		}
		err := s.doCall(method, base, path, username, body, out)
		if isUnavailable(err) {
			breaker.failure()
			return &UnavailableError{Service: s.serviceName(base)}
		}
		breaker.success()
		return err
	}

	err := s.doCall(method, base, path, username, body, out)
	if isUnavailable(err) {
		return &UnavailableError{Service: s.serviceName(base)}
	}
	return err
}

func (s *GatewayService) doCall(method, base, path, username string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-User-Name", username)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		var payload struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&payload)
		if payload.Message == "" {
			payload.Message = resp.Status
		}
		return &remoteError{status: resp.StatusCode, message: payload.Message}
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func isUnavailable(err error) bool {
	if err == nil {
		return false
	}
	var remote *remoteError
	return !errors.As(err, &remote) || remote.status >= http.StatusInternalServerError
}

func (s *GatewayService) Flights(page, size int) (model.PaginationResponse, error) {
	var result model.PaginationResponse
	return result, s.call(http.MethodGet, s.flightURL, "/internal/flights?page="+fmt.Sprint(page)+"&size="+fmt.Sprint(size), "", nil, &result)
}

func (s *GatewayService) flight(number string) (model.FlightResponse, error) {
	var result model.FlightResponse
	return result, s.call(http.MethodGet, s.flightURL, "/internal/flights/"+url.PathEscape(number), "", nil, &result)
}

func (s *GatewayService) GetBonus(username string) (model.PrivilegeInfoResponse, error) {
	var result model.PrivilegeInfoResponse
	return result, s.call(http.MethodGet, s.bonusURL, "/internal/bonus", username, nil, &result)
}

func (s *GatewayService) tickets(username string) ([]model.Ticket, error) {
	var result []model.Ticket
	return result, s.call(http.MethodGet, s.ticketURL, "/internal/tickets", username, nil, &result)
}

func (s *GatewayService) ticket(username, uid string) (model.Ticket, error) {
	var result model.Ticket
	return result, s.call(http.MethodGet, s.ticketURL, "/internal/tickets/"+url.PathEscape(uid), username, nil, &result)
}

func (s *GatewayService) ticketResponse(ticket model.Ticket) model.TicketResponse {
	flight, err := s.flight(ticket.FlightNumber)
	if err != nil {
		// Flight data enriches a ticket, but the ticket itself remains valid.
		return model.TicketResponse{TicketUID: ticket.TicketUID, FlightNumber: ticket.FlightNumber, Price: ticket.Price, Status: ticket.Status}
	}
	return model.TicketResponse{TicketUID: ticket.TicketUID, FlightNumber: ticket.FlightNumber, FromAirport: flight.FromAirport, ToAirport: flight.ToAirport, Date: flight.Date, Price: ticket.Price, Status: ticket.Status}
}

func (s *GatewayService) Tickets(username string) ([]model.TicketResponse, error) {
	tickets, err := s.tickets(username)
	if err != nil {
		return nil, err
	}
	result := make([]model.TicketResponse, 0, len(tickets))
	for _, ticket := range tickets {
		result = append(result, s.ticketResponse(ticket))
	}
	return result, nil
}

func (s *GatewayService) Ticket(username, uid string) (model.TicketResponse, error) {
	ticket, err := s.ticket(username, uid)
	if err != nil {
		return model.TicketResponse{}, err
	}
	return s.ticketResponse(ticket), nil
}

func (s *GatewayService) applyBonus(username string, operation model.PrivilegeOperationRequest) (model.PrivilegeShortInfo, error) {
	var result model.PrivilegeShortInfo
	return result, s.call(http.MethodPost, s.bonusURL, "/internal/bonus/operations", username, operation, &result)
}

func (s *GatewayService) rollbackTicket(username, ticketUID string) {
	_ = s.call(http.MethodDelete, s.ticketURL, "/internal/tickets/"+url.PathEscape(ticketUID), username, nil, nil)
}

func (s *GatewayService) Purchase(username string, req model.TicketPurchaseRequest) (model.TicketPurchaseResponse, error) {
	if username == "" || req.FlightNumber == "" || req.Price <= 0 {
		return model.TicketPurchaseResponse{}, fmt.Errorf("invalid ticket purchase request")
	}
	flight, err := s.flight(req.FlightNumber)
	if err != nil {
		return model.TicketPurchaseResponse{}, err
	}
	if req.Price != flight.Price {
		return model.TicketPurchaseResponse{}, fmt.Errorf("ticket price does not match flight price")
	}
	var balance model.PrivilegeInfoResponse
	if req.PaidFromBalance {
		balance, err = s.GetBonus(username)
		if err != nil {
			return model.TicketPurchaseResponse{}, err
		}
	}
	var ticket model.Ticket
	err = s.call(http.MethodPost, s.ticketURL, "/internal/tickets", username, map[string]any{"flightNumber": req.FlightNumber, "price": req.Price}, &ticket)
	if err != nil {
		return model.TicketPurchaseResponse{}, err
	}
	paidBonuses := 0
	operation := model.PrivilegeOperationRequest{TicketUID: ticket.TicketUID}
	if req.PaidFromBalance {
		paidBonuses = min(balance.Balance, req.Price)
		operation.Amount = paidBonuses
		operation.OperationType = model.PrivilegeOperationDebit
	} else {
		operation.Amount = req.Price / 10
		operation.OperationType = model.PrivilegeOperationFillInBalance
	}
	privilege, err := s.applyBonus(username, operation)
	if err != nil {
		s.rollbackTicket(username, ticket.TicketUID.String())
		return model.TicketPurchaseResponse{}, err
	}
	return model.TicketPurchaseResponse{TicketResponse: s.ticketResponse(ticket), PaidByMoney: req.Price - paidBonuses, PaidByBonuses: paidBonuses, Privilege: privilege}, nil
}

func (s *GatewayService) Cancel(username, uid string) error {
	ticket, err := s.ticket(username, uid)
	if err != nil {
		return err
	}
	if err = s.call(http.MethodPut, s.ticketURL, "/internal/tickets/"+url.PathEscape(uid)+"/cancel", username, nil, nil); err != nil {
		return err
	}
	if err = s.reverseBonus(username, ticket); err != nil {
		s.enqueueCancellation(cancelJob{username: username, ticket: ticket})
	}
	return nil
}

func (s *GatewayService) reverseBonus(username string, ticket model.Ticket) error {
	privilege, err := s.GetBonus(username)
	if err != nil {
		return err
	}
	for _, item := range privilege.History {
		if item.TicketUID != ticket.TicketUID {
			continue
		}
		operationType := model.PrivilegeOperationDebit
		if item.OperationType == model.PrivilegeOperationDebit {
			operationType = model.PrivilegeOperationFillInBalance
		}
		_, err = s.applyBonus(username, model.PrivilegeOperationRequest{TicketUID: ticket.TicketUID, Amount: abs(item.BalanceDiff), OperationType: operationType})
		return err
	}
	return nil
}

func (s *GatewayService) enqueueCancellation(job cancelJob) {
	select {
	case s.cancelQueue <- job:
	default:
		go func() { s.cancelQueue <- job }()
	}
}

func (s *GatewayService) processCancellationQueue() {
	for job := range s.cancelQueue {
		if err := s.reverseBonus(job.username, job.ticket); err != nil {
			time.AfterFunc(queueRetryTimeout, func() { s.enqueueCancellation(job) })
		}
	}
}

func (s *GatewayService) Me(username string) (model.UserInfoResponse, error) {
	tickets, err := s.Tickets(username)
	if err != nil {
		return model.UserInfoResponse{}, err
	}
	privilege, err := s.GetBonus(username)
	if err != nil {
		return model.UserInfoResponse{Tickets: tickets}, nil
	}
	return model.UserInfoResponse{Tickets: tickets, Privilege: model.PrivilegeShortInfo{Balance: privilege.Balance, Status: privilege.Status}}, nil
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
