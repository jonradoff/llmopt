package saas

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// These models mirror the LastSaaS MongoDB document shapes.
// They are read-only — llmopt never writes to these collections.

// BillingStatus represents the current billing state of a tenant.
type BillingStatus string

const (
	BillingStatusNone     BillingStatus = "none"
	BillingStatusActive   BillingStatus = "active"
	BillingStatusPastDue  BillingStatus = "past_due"
	BillingStatusCanceled BillingStatus = "canceled"
)

// Tenant mirrors the LastSaaS tenants collection.
type Tenant struct {
	ID                   primitive.ObjectID  `bson:"_id"`
	Name                 string              `bson:"name"`
	Slug                 string              `bson:"slug"`
	IsRoot               bool                `bson:"isRoot"`
	IsActive             bool                `bson:"isActive"`
	PlanID               *primitive.ObjectID `bson:"planId,omitempty"`
	BillingWaived        bool                `bson:"billingWaived"`
	SubscriptionCredits  int64               `bson:"subscriptionCredits"`
	PurchasedCredits     int64               `bson:"purchasedCredits"`
	StripeCustomerID     string              `bson:"stripeCustomerId,omitempty"`
	BillingStatus        BillingStatus       `bson:"billingStatus"`
	StripeSubscriptionID string              `bson:"stripeSubscriptionId,omitempty"`
	BillingInterval      string              `bson:"billingInterval,omitempty"`
	CurrentPeriodEnd     *time.Time          `bson:"currentPeriodEnd,omitempty"`
	CanceledAt           *time.Time          `bson:"canceledAt,omitempty"`
	TrialUsedAt          *time.Time          `bson:"trialUsedAt,omitempty"`
	SeatQuantity         int                 `bson:"seatQuantity"`
	CreatedAt            time.Time           `bson:"createdAt"`
	UpdatedAt            time.Time           `bson:"updatedAt"`
}

// TenantMembership mirrors the LastSaaS tenant_memberships collection.
type TenantMembership struct {
	ID       primitive.ObjectID `bson:"_id"`
	UserID   primitive.ObjectID `bson:"userId"`
	TenantID primitive.ObjectID `bson:"tenantId"`
	Role     string             `bson:"role"`
	JoinedAt time.Time          `bson:"joinedAt"`
}

// EntitlementType constants
const (
	EntitlementTypeBool    = "bool"
	EntitlementTypeNumeric = "numeric"
)

// EntitlementValue mirrors the LastSaaS plan entitlement value.
type EntitlementValue struct {
	Type         string `bson:"type"` // "bool" or "numeric"
	BoolValue    bool   `bson:"boolValue"`
	NumericValue int64  `bson:"numericValue"`
	Description  string `bson:"description"`
}

// PricingModel constants
type PricingModel string

const (
	PricingModelFlat    PricingModel = "flat"
	PricingModelPerSeat PricingModel = "per_seat"
)

// APIKey mirrors the LastSaaS api_keys collection (read-only).
type APIKey struct {
	ID        primitive.ObjectID `bson:"_id"`
	KeyHash   string             `bson:"keyHash"`
	Authority string             `bson:"authority"` // "admin" or "user"
	CreatedBy primitive.ObjectID `bson:"createdBy"`
	IsActive  bool               `bson:"isActive"`
}

// User mirrors the LastSaaS users collection (read-only).
type User struct {
	ID       primitive.ObjectID `bson:"_id"`
	Email    string             `bson:"email"`
	IsActive bool               `bson:"isActive"`
}

// Plan mirrors the LastSaaS plans collection.
type Plan struct {
	ID                   primitive.ObjectID          `bson:"_id"`
	Name                 string                      `bson:"name"`
	Description          string                      `bson:"description"`
	PricingModel         PricingModel                `bson:"pricingModel"`
	MonthlyPriceCents    int64                       `bson:"monthlyPriceCents"`
	AnnualDiscountPct    int                         `bson:"annualDiscountPct"`
	PerSeatPriceCents    int64                       `bson:"perSeatPriceCents"`
	IncludedSeats        int                         `bson:"includedSeats"`
	MinSeats             int                         `bson:"minSeats"`
	MaxSeats             int                         `bson:"maxSeats"`
	UsageCreditsPerMonth int64                       `bson:"usageCreditsPerMonth"`
	CreditResetPolicy    string                      `bson:"creditResetPolicy"`
	BonusCredits         int64                       `bson:"bonusCredits"`
	UserLimit            int                         `bson:"userLimit"`
	TrialDays            int                         `bson:"trialDays"`
	Entitlements         map[string]EntitlementValue `bson:"entitlements"`
	IsSystem             bool                        `bson:"isSystem"`
	IsArchived           bool                        `bson:"isArchived"`
	CreatedAt            time.Time                   `bson:"createdAt"`
	UpdatedAt            time.Time                   `bson:"updatedAt"`
}
