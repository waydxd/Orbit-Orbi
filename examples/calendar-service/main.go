package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	pb "github.com/waydxd/Orbit-Orbi/proto"
	"google.golang.org/grpc"
)

// CalendarServer is a simple in-memory implementation of the calendar service
type CalendarServer struct {
	pb.UnimplementedCalendarServiceServer
	mu     sync.RWMutex
	events map[string]*pb.Event
}

// NewCalendarServer creates a new calendar server
func NewCalendarServer() *CalendarServer {
	return &CalendarServer{
		events: make(map[string]*pb.Event),
	}
}

// CreateEvent implements the CreateEvent RPC
func (s *CalendarServer) CreateEvent(ctx context.Context, req *pb.CreateEventRequest) (*pb.CreateEventResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate a simple ID
	id := fmt.Sprintf("event-%d", len(s.events)+1)

	event := &pb.Event{
		Id:          id,
		Title:       req.Title,
		Description: req.Description,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Location:    req.Location,
		Attendees:   req.Attendees,
		Status:      "confirmed",
	}

	s.events[id] = event

	return &pb.CreateEventResponse{
		Event:   event,
		Success: true,
		Message: "Event created successfully",
	}, nil
}

// GetEvents implements the GetEvents RPC
func (s *CalendarServer) GetEvents(ctx context.Context, req *pb.GetEventsRequest) (*pb.GetEventsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var events []*pb.Event
	for _, event := range s.events {
		// Filter by time range
		if event.StartTime >= req.StartTime && event.EndTime <= req.EndTime {
			// Filter by status if provided
			if req.Status == "" || event.Status == req.Status {
				events = append(events, event)
			}
		}
	}

	return &pb.GetEventsResponse{
		Events:  events,
		Success: true,
		Message: fmt.Sprintf("Found %d events", len(events)),
	}, nil
}

// UpdateEvent implements the UpdateEvent RPC
func (s *CalendarServer) UpdateEvent(ctx context.Context, req *pb.UpdateEventRequest) (*pb.UpdateEventResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, exists := s.events[req.Id]
	if !exists {
		return &pb.UpdateEventResponse{
			Success: false,
			Message: "Event not found",
		}, nil
	}

	// Update fields if provided
	if req.Title != "" {
		event.Title = req.Title
	}
	if req.Description != "" {
		event.Description = req.Description
	}
	if req.StartTime != 0 {
		event.StartTime = req.StartTime
	}
	if req.EndTime != 0 {
		event.EndTime = req.EndTime
	}
	if req.Location != "" {
		event.Location = req.Location
	}
	if len(req.Attendees) > 0 {
		event.Attendees = req.Attendees
	}
	if req.Status != "" {
		event.Status = req.Status
	}

	return &pb.UpdateEventResponse{
		Event:   event,
		Success: true,
		Message: "Event updated successfully",
	}, nil
}

// DeleteEvent implements the DeleteEvent RPC
func (s *CalendarServer) DeleteEvent(ctx context.Context, req *pb.DeleteEventRequest) (*pb.DeleteEventResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.events[req.Id]; !exists {
		return &pb.DeleteEventResponse{
			Success: false,
			Message: "Event not found",
		}, nil
	}

	delete(s.events, req.Id)

	return &pb.DeleteEventResponse{
		Success: true,
		Message: "Event deleted successfully",
	}, nil
}

// GetAvailableSlots implements the GetAvailableSlots RPC
func (s *CalendarServer) GetAvailableSlots(ctx context.Context, req *pb.GetAvailableSlotsRequest) (*pb.GetAvailableSlotsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Simple implementation: find gaps between events
	var slots []*pb.TimeSlot

	// For this example, we'll just return some sample slots
	// A real implementation would check for conflicts with existing events
	currentTime := req.StartTime
	for currentTime+req.Duration <= req.EndTime {
		slots = append(slots, &pb.TimeSlot{
			StartTime: currentTime,
			EndTime:   currentTime + req.Duration,
		})
		currentTime += req.Duration
	}

	return &pb.GetAvailableSlotsResponse{
		Slots:   slots,
		Success: true,
		Message: fmt.Sprintf("Found %d available slots", len(slots)),
	}, nil
}

func main() {
	// Create a listener on TCP port 50051
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Create a gRPC server
	grpcServer := grpc.NewServer()

	// Register the calendar service
	pb.RegisterCalendarServiceServer(grpcServer, NewCalendarServer())

	log.Println("Calendar service starting on port 50051...")

	// Start serving
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
