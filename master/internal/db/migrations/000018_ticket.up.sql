-- 工单表
CREATE TABLE IF NOT EXISTS tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_no VARCHAR(32) UNIQUE NOT NULL,
    user_id BIGINT NOT NULL,
    subject VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'other',
    priority VARCHAR(16) NOT NULL DEFAULT 'normal',
    status VARCHAR(16) NOT NULL DEFAULT 'open',
    product_id UUID,
    instance_id UUID,
    assignee_id BIGINT,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_product_id ON tickets(product_id);
CREATE INDEX IF NOT EXISTS idx_tickets_instance_id ON tickets(instance_id);
CREATE INDEX IF NOT EXISTS idx_tickets_assignee_id ON tickets(assignee_id);
CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status);
CREATE INDEX IF NOT EXISTS idx_tickets_deleted_at ON tickets(deleted_at);

-- 工单回复表
CREATE TABLE IF NOT EXISTS ticket_replies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID NOT NULL,
    user_id BIGINT NOT NULL,
    content TEXT NOT NULL,
    is_staff BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ticket_replies_ticket_id ON ticket_replies(ticket_id);
CREATE INDEX IF NOT EXISTS idx_ticket_replies_user_id ON ticket_replies(user_id);
CREATE INDEX IF NOT EXISTS idx_ticket_replies_deleted_at ON ticket_replies(deleted_at);
