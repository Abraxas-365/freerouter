-- Prevent double-crediting from concurrent Stripe webhook deliveries:
-- a given Stripe checkout session may only ever produce one top_up transaction.
CREATE UNIQUE INDEX IF NOT EXISTS uq_credit_tx_stripe_reference
    ON credit_transactions (reference_id)
    WHERE reference_id LIKE 'stripe:%';
