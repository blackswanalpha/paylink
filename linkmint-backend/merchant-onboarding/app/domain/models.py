"""Domain value objects (enums). Pure, no I/O."""

from __future__ import annotations

from enum import StrEnum


class MerchantStatus(StrEnum):
    DRAFT = "DRAFT"
    PENDING_VERIFICATION = "PENDING_VERIFICATION"
    ACTIVE = "ACTIVE"
    REJECTED = "REJECTED"
    SUSPENDED = "SUSPENDED"


class MerchantType(StrEnum):
    INDIVIDUAL = "individual"
    COMPANY = "company"
    NONPROFIT = "nonprofit"


class BankAccountStatus(StrEnum):
    PENDING_VERIFY = "PENDING_VERIFY"
    VERIFIED = "VERIFIED"
    REVOKED = "REVOKED"


class Rail(StrEnum):
    """Settlement rails a bank account can be linked to (rail-agnostic at the protocol boundary)."""

    MPESA = "mpesa"
    SWIFT = "swift"
    SEPA = "sepa"
    ACH = "ach"
    CRYPTO = "crypto"


class FeeTier(StrEnum):
    """Fee tiers a merchant can be assigned. work21 (fee-pricing) consumes the tier."""

    STANDARD = "standard"
    STARTUP = "startup"
    ENTERPRISE = "enterprise"


class DocumentKind(StrEnum):
    CERT_INCORPORATION = "cert_incorporation"
    TAX_ID = "tax_id"
    DIRECTOR_ID = "director_id"
    PROOF_OF_ADDRESS = "proof_of_address"
    BANK_STATEMENT = "bank_statement"
    OTHER = "other"


class ReviewDecision(StrEnum):
    """A manual-review / consumer decision that drives the state machine via ``decide()``."""

    APPROVE = "approve"
    REJECT = "reject"
    SUSPEND = "suspend"
    REINSTATE = "reinstate"
