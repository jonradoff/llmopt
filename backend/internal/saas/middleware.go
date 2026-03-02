package saas

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type contextKey string

const (
	ctxUserID     contextKey = "saas_user_id"
	ctxEmail      contextKey = "saas_email"
	ctxTenantID   contextKey = "saas_tenant_id"
	ctxTenant     contextKey = "saas_tenant"
	ctxPlan       contextKey = "saas_plan"
	ctxMemberRole contextKey = "saas_member_role"
	ctxAuthMethod contextKey = "saas_auth_method"
)

// Middleware provides JWT auth and tenant resolution for SaaS routes.
type Middleware struct {
	jwt     *JWTValidator
	tenants *mongo.Collection
	members *mongo.Collection
	plans   *mongo.Collection
	apiKeys *mongo.Collection
	users   *mongo.Collection
}

func NewMiddleware(jwt *JWTValidator, db *mongo.Database) *Middleware {
	return &Middleware{
		jwt:     jwt,
		tenants: db.Collection("tenants"),
		members: db.Collection("tenant_memberships"),
		plans:   db.Collection("plans"),
		apiKeys: db.Collection("api_keys"),
		users:   db.Collection("users"),
	}
}

// RequireJWT validates the Authorization header and puts user info in context.
func (m *Middleware) RequireJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := m.jwt.Validate(token)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxEmail, claims.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireTenant reads X-Tenant-ID, verifies membership, and loads tenant + plan.
func (m *Middleware) RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantIDStr := r.Header.Get("X-Tenant-ID")
		if tenantIDStr == "" {
			http.Error(w, `{"error":"missing X-Tenant-ID header"}`, http.StatusBadRequest)
			return
		}

		tenantOID, err := primitive.ObjectIDFromHex(tenantIDStr)
		if err != nil {
			http.Error(w, `{"error":"invalid tenant ID"}`, http.StatusBadRequest)
			return
		}

		userIDStr := UserIDFromContext(r.Context())
		if userIDStr == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		userOID, err := primitive.ObjectIDFromHex(userIDStr)
		if err != nil {
			http.Error(w, `{"error":"invalid user ID"}`, http.StatusBadRequest)
			return
		}

		// Verify user is a member of this tenant
		var membership TenantMembership
		err = m.members.FindOne(r.Context(), bson.M{
			"userId":   userOID,
			"tenantId": tenantOID,
		}).Decode(&membership)
		if err != nil {
			http.Error(w, `{"error":"not a member of this tenant"}`, http.StatusForbidden)
			return
		}

		// Load tenant
		var tenant Tenant
		err = m.tenants.FindOne(r.Context(), bson.M{"_id": tenantOID}).Decode(&tenant)
		if err != nil {
			http.Error(w, `{"error":"tenant not found"}`, http.StatusNotFound)
			return
		}

		if !tenant.IsActive {
			http.Error(w, `{"error":"tenant is deactivated"}`, http.StatusForbidden)
			return
		}

		// Load plan if assigned
		var plan *Plan
		if tenant.PlanID != nil {
			var p Plan
			if err := m.plans.FindOne(r.Context(), bson.M{"_id": *tenant.PlanID}).Decode(&p); err == nil {
				plan = &p
			}
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxTenantID, tenantIDStr)
		ctx = context.WithValue(ctx, ctxTenant, &tenant)
		ctx = context.WithValue(ctx, ctxPlan, plan)
		ctx = context.WithValue(ctx, ctxMemberRole, membership.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Context helpers

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserID).(string)
	return v
}

func TenantIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxTenantID).(string)
	return v
}

func TenantFromContext(ctx context.Context) *Tenant {
	v, _ := ctx.Value(ctxTenant).(*Tenant)
	return v
}

func PlanFromContext(ctx context.Context) *Plan {
	v, _ := ctx.Value(ctxPlan).(*Plan)
	return v
}

func MemberRoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxMemberRole).(string)
	return v
}

func AuthMethodFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxAuthMethod).(string)
	return v
}

// RequireAuth validates both JWT tokens and lsk_ API keys, then resolves tenant context.
// For admin API keys, tenant auto-resolves to root. For user keys/JWTs, X-Tenant-ID is required.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, `{"error":"unauthorized","code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")

		if strings.HasPrefix(token, "lsk_") {
			m.authenticateAPIKey(w, r, next, token)
			return
		}

		// JWT path: validate token, then resolve tenant
		claims, err := m.jwt.Validate(token)
		if err != nil {
			http.Error(w, `{"error":"unauthorized","code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxEmail, claims.Email)
		ctx = context.WithValue(ctx, ctxAuthMethod, "jwt")

		// Resolve tenant from X-Tenant-ID header
		tenantIDStr := r.Header.Get("X-Tenant-ID")
		if tenantIDStr == "" {
			http.Error(w, `{"error":"missing X-Tenant-ID header","code":"BAD_REQUEST"}`, http.StatusBadRequest)
			return
		}

		ctx, err = m.resolveTenant(ctx, claims.UserID, tenantIDStr)
		if err != nil {
			status := http.StatusForbidden
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			} else if strings.Contains(err.Error(), "invalid") {
				status = http.StatusBadRequest
			}
			http.Error(w, fmt.Sprintf(`{"error":"%s","code":"FORBIDDEN"}`, err.Error()), status)
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) authenticateAPIKey(w http.ResponseWriter, r *http.Request, next http.Handler, rawKey string) {
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := base64.RawURLEncoding.EncodeToString(hash[:])

	ctx := r.Context()

	var apiKey APIKey
	err := m.apiKeys.FindOne(ctx, bson.M{"keyHash": keyHash, "isActive": true}).Decode(&apiKey)
	if err != nil {
		http.Error(w, `{"error":"unauthorized","code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
		return
	}

	// Load key creator
	var user User
	err = m.users.FindOne(ctx, bson.M{"_id": apiKey.CreatedBy}).Decode(&user)
	if err != nil || !user.IsActive {
		http.Error(w, `{"error":"unauthorized","code":"UNAUTHORIZED"}`, http.StatusUnauthorized)
		return
	}

	ctx = context.WithValue(ctx, ctxUserID, user.ID.Hex())
	ctx = context.WithValue(ctx, ctxEmail, user.Email)
	ctx = context.WithValue(ctx, ctxAuthMethod, "apikey")

	if apiKey.Authority == "admin" {
		// Admin keys auto-resolve to root tenant
		var rootTenant Tenant
		err = m.tenants.FindOne(ctx, bson.M{"isRoot": true}).Decode(&rootTenant)
		if err != nil {
			http.Error(w, `{"error":"root tenant not found"}`, http.StatusInternalServerError)
			return
		}
		ctx = context.WithValue(ctx, ctxTenantID, rootTenant.ID.Hex())
		ctx = context.WithValue(ctx, ctxTenant, &rootTenant)
		ctx = context.WithValue(ctx, ctxMemberRole, "admin")

		// Load plan if assigned
		if rootTenant.PlanID != nil {
			var p Plan
			if m.plans.FindOne(ctx, bson.M{"_id": *rootTenant.PlanID}).Decode(&p) == nil {
				ctx = context.WithValue(ctx, ctxPlan, &p)
			}
		}
	} else {
		// User keys require X-Tenant-ID header
		tenantIDStr := r.Header.Get("X-Tenant-ID")
		if tenantIDStr == "" {
			http.Error(w, `{"error":"X-Tenant-ID header required for user API keys","code":"BAD_REQUEST"}`, http.StatusBadRequest)
			return
		}
		var resolveErr error
		ctx, resolveErr = m.resolveTenant(ctx, user.ID.Hex(), tenantIDStr)
		if resolveErr != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s","code":"FORBIDDEN"}`, resolveErr.Error()), http.StatusForbidden)
			return
		}
	}

	// Async update lastUsedAt
	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.apiKeys.UpdateOne(updateCtx, bson.M{"_id": apiKey.ID}, bson.M{"$set": bson.M{"lastUsedAt": time.Now()}})
	}()

	next.ServeHTTP(w, r.WithContext(ctx))
}

// resolveTenant loads tenant + membership + plan into context for a given user and tenant ID.
func (m *Middleware) resolveTenant(ctx context.Context, userIDStr, tenantIDStr string) (context.Context, error) {
	tenantOID, err := primitive.ObjectIDFromHex(tenantIDStr)
	if err != nil {
		return ctx, fmt.Errorf("invalid tenant ID")
	}
	userOID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return ctx, fmt.Errorf("invalid user ID")
	}

	var membership TenantMembership
	err = m.members.FindOne(ctx, bson.M{"userId": userOID, "tenantId": tenantOID}).Decode(&membership)
	if err != nil {
		return ctx, fmt.Errorf("not a member of this tenant")
	}

	var tenant Tenant
	err = m.tenants.FindOne(ctx, bson.M{"_id": tenantOID}).Decode(&tenant)
	if err != nil {
		return ctx, fmt.Errorf("tenant not found")
	}
	if !tenant.IsActive {
		return ctx, fmt.Errorf("tenant is deactivated")
	}

	var plan *Plan
	if tenant.PlanID != nil {
		var p Plan
		if m.plans.FindOne(ctx, bson.M{"_id": *tenant.PlanID}).Decode(&p) == nil {
			plan = &p
		}
	}

	ctx = context.WithValue(ctx, ctxTenantID, tenantIDStr)
	ctx = context.WithValue(ctx, ctxTenant, &tenant)
	ctx = context.WithValue(ctx, ctxPlan, plan)
	ctx = context.WithValue(ctx, ctxMemberRole, membership.Role)
	return ctx, nil
}

// AuthInfo holds the resolved authentication information for a token.
type AuthInfo struct {
	UserID   string
	Email    string
	TenantID string
	Tenant   *Tenant
	Plan     *Plan
	Role     string
	Method   string // "apikey", "jwt", or "mcp_token"
}

// ValidateToken checks a bearer token (lsk_ API key or JWT) and returns auth info.
// tenantIDHint is the X-Tenant-ID header value (required for user-authority keys and JWTs).
func (m *Middleware) ValidateToken(ctx context.Context, token, tenantIDHint string) (*AuthInfo, error) {
	if strings.HasPrefix(token, "lsk_") {
		return m.validateAPIKey(ctx, token, tenantIDHint)
	}

	// JWT path
	claims, err := m.jwt.Validate(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token")
	}

	if tenantIDHint == "" {
		return nil, fmt.Errorf("missing tenant ID")
	}

	info := &AuthInfo{
		UserID: claims.UserID,
		Email:  claims.Email,
		Method: "jwt",
	}

	// Resolve tenant
	if err := m.resolveAuthInfoTenant(ctx, info, tenantIDHint); err != nil {
		return nil, err
	}
	return info, nil
}

func (m *Middleware) validateAPIKey(ctx context.Context, rawKey, tenantIDHint string) (*AuthInfo, error) {
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := base64.RawURLEncoding.EncodeToString(hash[:])

	var apiKey APIKey
	if err := m.apiKeys.FindOne(ctx, bson.M{"keyHash": keyHash, "isActive": true}).Decode(&apiKey); err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	var user User
	if err := m.users.FindOne(ctx, bson.M{"_id": apiKey.CreatedBy}).Decode(&user); err != nil || !user.IsActive {
		return nil, fmt.Errorf("user not active")
	}

	info := &AuthInfo{
		UserID: user.ID.Hex(),
		Email:  user.Email,
		Method: "apikey",
	}

	if apiKey.Authority == "admin" {
		var rootTenant Tenant
		if err := m.tenants.FindOne(ctx, bson.M{"isRoot": true}).Decode(&rootTenant); err != nil {
			return nil, fmt.Errorf("root tenant not found")
		}
		info.TenantID = rootTenant.ID.Hex()
		info.Tenant = &rootTenant
		info.Role = "admin"
		if rootTenant.PlanID != nil {
			var p Plan
			if m.plans.FindOne(ctx, bson.M{"_id": *rootTenant.PlanID}).Decode(&p) == nil {
				info.Plan = &p
			}
		}
	} else {
		if tenantIDHint == "" {
			return nil, fmt.Errorf("X-Tenant-ID header required for user API keys")
		}
		if err := m.resolveAuthInfoTenant(ctx, info, tenantIDHint); err != nil {
			return nil, err
		}
	}

	// Async update lastUsedAt
	go func() {
		updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.apiKeys.UpdateOne(updateCtx, bson.M{"_id": apiKey.ID}, bson.M{"$set": bson.M{"lastUsedAt": time.Now()}})
	}()

	return info, nil
}

// resolveAuthInfoTenant loads tenant, membership, and plan into the AuthInfo.
func (m *Middleware) resolveAuthInfoTenant(ctx context.Context, info *AuthInfo, tenantIDStr string) error {
	tenantOID, err := primitive.ObjectIDFromHex(tenantIDStr)
	if err != nil {
		return fmt.Errorf("invalid tenant ID")
	}
	userOID, err := primitive.ObjectIDFromHex(info.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID")
	}

	var membership TenantMembership
	if err := m.members.FindOne(ctx, bson.M{"userId": userOID, "tenantId": tenantOID}).Decode(&membership); err != nil {
		return fmt.Errorf("not a member of this tenant")
	}

	var tenant Tenant
	if err := m.tenants.FindOne(ctx, bson.M{"_id": tenantOID}).Decode(&tenant); err != nil {
		return fmt.Errorf("tenant not found")
	}
	if !tenant.IsActive {
		return fmt.Errorf("tenant is deactivated")
	}

	info.TenantID = tenantIDStr
	info.Tenant = &tenant
	info.Role = membership.Role
	if tenant.PlanID != nil {
		var p Plan
		if m.plans.FindOne(ctx, bson.M{"_id": *tenant.PlanID}).Decode(&p) == nil {
			info.Plan = &p
		}
	}
	return nil
}

// SetAuthContext populates the request context with auth values from AuthInfo.
func SetAuthContext(ctx context.Context, info *AuthInfo) context.Context {
	ctx = context.WithValue(ctx, ctxUserID, info.UserID)
	ctx = context.WithValue(ctx, ctxEmail, info.Email)
	ctx = context.WithValue(ctx, ctxAuthMethod, info.Method)
	ctx = context.WithValue(ctx, ctxTenantID, info.TenantID)
	if info.Tenant != nil {
		ctx = context.WithValue(ctx, ctxTenant, info.Tenant)
	}
	if info.Plan != nil {
		ctx = context.WithValue(ctx, ctxPlan, info.Plan)
	}
	ctx = context.WithValue(ctx, ctxMemberRole, info.Role)
	return ctx
}

// RequireActiveBilling returns middleware that blocks requests when the tenant's
// billing status is not active (and not waived/root). Prevents users from
// accessing paid features after subscription expiration or cancellation.
func RequireActiveBilling() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := TenantFromContext(r.Context())
			if tenant == nil {
				http.Error(w, `{"error":"no tenant context"}`, http.StatusBadRequest)
				return
			}

			// Root tenant and billing-waived tenants are exempt
			if tenant.IsRoot || tenant.BillingWaived {
				next.ServeHTTP(w, r)
				return
			}

			// Allow if billing status is active or none (free/unsubscribed tenants)
			if tenant.BillingStatus == BillingStatusActive || tenant.BillingStatus == BillingStatusNone {
				next.ServeHTTP(w, r)
				return
			}

			http.Error(w, `{"error":"subscription_required","code":"BILLING_INACTIVE"}`, http.StatusPaymentRequired)
		})
	}
}

// RequireEntitlement returns middleware that checks whether the tenant's plan
// grants a specific boolean entitlement. Requires the plan to already be loaded
// in context by RequireTenant.
func RequireEntitlement(plansCol *mongo.Collection, feature string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := TenantFromContext(r.Context())
			if tenant == nil {
				http.Error(w, `{"error":"no tenant context"}`, http.StatusBadRequest)
				return
			}

			// Root tenant and billing-waived tenants get all features
			if tenant.IsRoot || tenant.BillingWaived {
				next.ServeHTTP(w, r)
				return
			}

			if tenant.PlanID == nil {
				http.Error(w, fmt.Sprintf(`{"error":"feature '%s' requires an active plan","code":"ENTITLEMENT_REQUIRED"}`, feature), http.StatusForbidden)
				return
			}

			var plan Plan
			if err := plansCol.FindOne(r.Context(), bson.M{"_id": *tenant.PlanID}).Decode(&plan); err != nil {
				http.Error(w, `{"error":"plan not found"}`, http.StatusInternalServerError)
				return
			}

			ent, exists := plan.Entitlements[feature]
			if !exists || (ent.Type == EntitlementTypeBool && !ent.BoolValue) {
				http.Error(w, fmt.Sprintf(`{"error":"feature '%s' is not included in your plan","code":"ENTITLEMENT_REQUIRED"}`, feature), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
