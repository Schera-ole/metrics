package grpcserver

import (
	"context"
	"net/netip"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func XRequestIPInterceptor(trustedSubnet string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Internal, "cannot get metadata")
		}
		realIPs := md.Get("x-real-ip")
		if len(realIPs) == 0 || realIPs[0] == "" {
			return nil, status.Error(codes.InvalidArgument, "missing required header: x-real-ip")
		}
		clientIP := realIPs[0]
		if trustedSubnet != "" {
			if !isIPinSubnet(clientIP, trustedSubnet) {
				return nil, status.Error(codes.PermissionDenied, "client IP not in trusted subnet")
			}
		}
		return handler(ctx, req)
	}
}

func isIPinSubnet(clientIP string, trustedSubnet string) bool {
	clientAddr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return false
	}
	subnet, err := netip.ParsePrefix(trustedSubnet)
	if err != nil {
		return false
	}
	return subnet.Contains(clientAddr)
}
