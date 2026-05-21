package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/devflex/traffoflex/packages/go-shared/ids"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"

	StatusActive          = "active"
	StatusPendingApproval = "pending_approval"
)

var (
	ErrInvalidInput    = errors.New("invalid auth input")
	ErrInvalidOTP      = errors.New("invalid or expired otp")
	ErrInvalidToken    = errors.New("invalid auth token")
	ErrPendingApproval = errors.New("user pending admin approval")
	ErrForbidden       = errors.New("forbidden")
	ErrNotFound        = errors.New("user not found")
	ErrOTPActive       = errors.New("otp is still active")
	ErrOTPRateLimited  = errors.New("otp request rate limited")
)

type User struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	Status         string    `json:"status"`
	EmailVerified  bool      `json:"email_verified"`
	Approved       bool      `json:"approved"`
	OTPHash        string    `json:"-"`
	OTPExpiresAt   time.Time `json:"-"`
	OTPRequestedAt time.Time `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Repository interface {
	ListUsers(ctx context.Context) (
		[]User,
		error,
	)
	GetUser(
		ctx context.Context,
		id string,
	) (
		User,
		error,
	)
	GetUserByEmail(
		ctx context.Context,
		email string,
	) (
		User,
		error,
	)
	SaveUser(
		ctx context.Context,
		user User,
	) (
		User,
		error,
	)
}

type Config struct {
	AdminEmail   string
	JWTSecret    string
	OTPTTL       time.Duration
	OTPRateLimit time.Duration
	SessionTTL   time.Duration
	DevReturnOTP bool
}

type OTPChallenge struct {
	Email string `json:"email"`
	OTP   string `json:"otp,omitempty"`
}

type Session struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type Service struct {
	repo     Repository
	delivery OTPDelivery
	cfg      Config
	now      func() time.Time
}

func NewService(
	repo Repository,
	cfg Config,
) *Service {
	return NewServiceWithDelivery(
		repo,
		nil,
		cfg,
	)
}

func NewServiceWithDelivery(
	repo Repository,
	delivery OTPDelivery,
	cfg Config,
) *Service {
	if cfg.OTPRateLimit <= 0 {
		cfg.OTPRateLimit = time.Minute
	}
	return &Service{
		repo:     repo,
		delivery: delivery,
		cfg:      cfg,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) RequestOTP(
	ctx context.Context,
	email string,
) (
	OTPChallenge,
	error,
) {
	email = normalizeEmail(email)
	if err := validateEmail(email); err != nil {
		return OTPChallenge{}, err
	}

	user, err := s.repo.GetUserByEmail(
		ctx,
		email,
	)
	if err != nil {
		if !errors.Is(
			err,
			ErrNotFound,
		) {
			return OTPChallenge{}, err
		}
		user = s.newUser(email)
	}

	s.applyAdminPolicy(&user)
	now := s.now()
	if user.OTPHash != "" && now.Before(user.OTPExpiresAt) {
		return OTPChallenge{}, ErrOTPActive
	}
	if !user.OTPRequestedAt.IsZero() && now.Sub(user.OTPRequestedAt) < s.cfg.OTPRateLimit {
		return OTPChallenge{}, ErrOTPRateLimited
	}
	otp, err := generateOTP()
	if err != nil {
		return OTPChallenge{}, err
	}
	user.OTPHash = s.hashOTP(
		email,
		otp,
	)
	user.OTPExpiresAt = now.Add(s.cfg.OTPTTL)
	user.OTPRequestedAt = now
	user.UpdatedAt = now
	if _, err := s.repo.SaveUser(
		ctx,
		user,
	); err != nil {
		return OTPChallenge{}, err
	}
	if s.delivery != nil {
		if err := s.delivery.SendOTP(
			ctx,
			email,
			otp,
		); err != nil {
			user.OTPHash = ""
			user.OTPExpiresAt = time.Time{}
			user.UpdatedAt = s.now()
			if _, rollbackErr := s.repo.SaveUser(
				ctx,
				user,
			); rollbackErr != nil {
				return OTPChallenge{}, errors.Join(
					err,
					rollbackErr,
				)
			}
			return OTPChallenge{}, err
		}
	}

	challenge := OTPChallenge{Email: email}
	if s.cfg.DevReturnOTP {
		challenge.OTP = otp
	}
	return challenge, nil
}

func (s *Service) VerifyOTP(
	ctx context.Context,
	email,
	otp string,
) (
	Session,
	error,
) {
	email = normalizeEmail(email)
	otp = strings.TrimSpace(otp)
	if err := validateEmail(email); err != nil {
		return Session{}, err
	}
	if otp == "" {
		return Session{}, errors.Join(
			ErrInvalidInput,
			errors.New("otp is required"),
		)
	}

	user, err := s.repo.GetUserByEmail(
		ctx,
		email,
	)
	if err != nil {
		return Session{}, err
	}
	if user.OTPHash == "" || s.now().After(user.OTPExpiresAt) {
		return Session{}, ErrInvalidOTP
	}
	if !hmac.Equal(
		[]byte(user.OTPHash),
		[]byte(s.hashOTP(
			email,
			otp,
		)),
	) {
		return Session{}, ErrInvalidOTP
	}

	s.applyAdminPolicy(&user)
	user.EmailVerified = true
	user.OTPHash = ""
	user.OTPExpiresAt = time.Time{}
	user.UpdatedAt = s.now()
	user, err = s.repo.SaveUser(
		ctx,
		user,
	)
	if err != nil {
		return Session{}, err
	}
	if !user.Approved {
		return Session{User: user}, ErrPendingApproval
	}

	token, err := s.signToken(user)
	if err != nil {
		return Session{}, err
	}
	return Session{
		Token: token,
		User:  user,
	}, nil
}

func (s *Service) Authenticate(
	ctx context.Context,
	token string,
) (
	User,
	error,
) {
	claims, err := s.verifyToken(token)
	if err != nil {
		return User{}, err
	}
	user, err := s.repo.GetUser(
		ctx,
		claims.Subject,
	)
	if err != nil {
		return User{}, err
	}
	if !user.Approved {
		return User{}, ErrPendingApproval
	}
	return user, nil
}

func (s *Service) ListUsers(ctx context.Context) (
	[]User,
	error,
) {
	return s.repo.ListUsers(ctx)
}

func (s *Service) ApproveUser(
	ctx context.Context,
	id string,
) (
	User,
	error,
) {
	user, err := s.repo.GetUser(
		ctx,
		strings.TrimSpace(id),
	)
	if err != nil {
		return User{}, err
	}
	user.Approved = true
	user.Status = StatusActive
	user.UpdatedAt = s.now()
	return s.repo.SaveUser(
		ctx,
		user,
	)
}

func (s *Service) newUser(email string) User {
	now := s.now()
	user := User{
		ID:            ids.New("usr"),
		Email:         email,
		Role:          RoleUser,
		Status:        StatusPendingApproval,
		EmailVerified: false,
		Approved:      false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.applyAdminPolicy(&user)
	return user
}

func (s *Service) applyAdminPolicy(user *User) {
	if normalizeEmail(user.Email) != normalizeEmail(s.cfg.AdminEmail) {
		if user.Role == "" {
			user.Role = RoleUser
		}
		if user.Approved {
			user.Status = StatusActive
		} else {
			user.Status = StatusPendingApproval
		}
		return
	}
	user.Role = RoleAdmin
	user.Approved = true
	user.Status = StatusActive
}

func (s *Service) hashOTP(
	email,
	otp string,
) string {
	mac := hmac.New(
		sha256.New,
		[]byte(s.cfg.JWTSecret),
	)
	mac.Write([]byte(email))
	mac.Write([]byte(":"))
	mac.Write([]byte(otp))
	return hex.EncodeToString(mac.Sum(nil))
}

type tokenHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

type tokenClaims struct {
	Subject  string `json:"sub"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Approved bool   `json:"approved"`
	Expires  int64  `json:"exp"`
}

func (s *Service) signToken(user User) (
	string,
	error,
) {
	header := tokenHeader{
		Alg: "HS256",
		Typ: "JWT",
	}
	claims := tokenClaims{
		Subject:  user.ID,
		Email:    user.Email,
		Role:     user.Role,
		Approved: user.Approved,
		Expires:  s.now().Add(s.cfg.SessionTTL).Unix(),
	}
	headerPart, err := encodeTokenPart(header)
	if err != nil {
		return "",
			err
	}
	claimsPart, err := encodeTokenPart(claims)
	if err != nil {
		return "",
			err
	}
	unsigned := headerPart + "." + claimsPart
	signature := s.sign(unsigned)
	return unsigned + "." + signature, nil
}

func (s *Service) verifyToken(token string) (
	tokenClaims,
	error,
) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return tokenClaims{}, ErrInvalidToken
	}
	unsigned := parts[0] + "." + parts[1]
	expected := s.sign(unsigned)
	if !hmac.Equal(
		[]byte(expected),
		[]byte(parts[2]),
	) {
		return tokenClaims{}, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return tokenClaims{}, ErrInvalidToken
	}
	var claims tokenClaims
	if err := json.Unmarshal(
		payload,
		&claims,
	); err != nil {
		return tokenClaims{}, ErrInvalidToken
	}
	if claims.Expires <= s.now().Unix() {
		return tokenClaims{}, ErrInvalidToken
	}
	return claims, nil
}

func (s *Service) sign(unsigned string) string {
	mac := hmac.New(
		sha256.New,
		[]byte(s.cfg.JWTSecret),
	)
	mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func encodeTokenPart(value any) (
	string,
	error,
) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "",
			err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateEmail(email string) error {
	if email == "" || !strings.Contains(email, "@") {
		return errors.Join(
			ErrInvalidInput,
			errors.New("valid email is required"),
		)
	}
	return nil
}

func generateOTP() (
	string,
	error,
) {
	max := big.NewInt(1000000)
	value, err := rand.Int(
		rand.Reader,
		max,
	)
	if err != nil {
		return "",
			err
	}
	return fmt.Sprintf(
		"%06d",
		value.Int64(),
	), nil
}
