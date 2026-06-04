// Package repository — vendor repository.
// Implementa CRUD de vendedores com pgx/v5.
// Constitution P-IV RBAC: verificações de papel ficam na camada service.
// Constitution P-V LGPD: Anonymize apaga PII e registra em audit_trail.
// CHK020 OWASP: todos os parâmetros usam bind params — zero interpolação SQL.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ─── Erros sentinela ──────────────────────────────────────────────────────────

var (
	// ErrVendorNotFound é retornado quando o vendedor não existe.
	ErrVendorNotFound = errors.New("repository: vendedor não encontrado")

	// ErrVendorEmailConflict é retornado quando o email já está em uso.
	ErrVendorEmailConflict = errors.New("repository: email de vendedor já cadastrado")
)

// ─── Domain types ─────────────────────────────────────────────────────────────

// VendorStatus representa o estado do vendedor.
type VendorStatus string

const (
	VendorStatusAtivo   VendorStatus = "ativo"
	VendorStatusInativo VendorStatus = "inativo"
)

// Vendor representa um vendedor conforme data-model.md §vendors.
type Vendor struct {
	ID           string
	Name         string
	Email        string
	Status       VendorStatus
	AnonymizedAt *time.Time
	CreatedAt    time.Time
}

// VendorFilter filtra listagens de vendedores.
type VendorFilter struct {
	Status *VendorStatus // nil = todos
}

// VendorPatch representa os campos mutáveis de um vendedor.
// Allowlist explícita — apenas name, email, status (CHK020 mass-assign protection).
type VendorPatch struct {
	Name   *string
	Email  *string
	Status *VendorStatus
}

// ─── Interface ────────────────────────────────────────────────────────────────

// VendorRepository define as operações de persistência de vendedores.
type VendorRepository interface {
	Create(ctx context.Context, vendor Vendor) (Vendor, error)
	FindByID(ctx context.Context, id string) (Vendor, error)
	List(ctx context.Context, filter VendorFilter) ([]Vendor, error)
	Update(ctx context.Context, id string, patch VendorPatch) (Vendor, error)
	Anonymize(ctx context.Context, id string, actorID string) error
}

// ─── Implementação PostgreSQL ─────────────────────────────────────────────────

// PGVendorRepository implementa VendorRepository com pgx/v5.
type PGVendorRepository struct {
	pool *pgxpool.Pool
}

// NewPGVendorRepository cria um PGVendorRepository.
func NewPGVendorRepository(pool *pgxpool.Pool) *PGVendorRepository {
	return &PGVendorRepository{pool: pool}
}

// ─── Create ───────────────────────────────────────────────────────────────────

// Create insere um novo vendedor. O ID é gerado pelo banco (gen_random_uuid()).
// Retorna ErrVendorEmailConflict se o email já estiver em uso.
func (r *PGVendorRepository) Create(ctx context.Context, vendor Vendor) (Vendor, error) {
	const q = `
		INSERT INTO vendors (name, email, status)
		VALUES ($1, $2, $3)
		RETURNING id, name, email, status, anonymized_at, created_at
	`
	status := vendor.Status
	if status == "" {
		status = VendorStatusAtivo
	}

	var v Vendor
	err := r.pool.QueryRow(ctx, q, vendor.Name, vendor.Email, status).Scan(
		&v.ID, &v.Name, &v.Email, &v.Status, &v.AnonymizedAt, &v.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err, "vendors_email_key") || isUniqueViolation(err, "vendors_email_unique") || isUniqueViolationMsg(err, "email") {
			return Vendor{}, ErrVendorEmailConflict
		}
		return Vendor{}, fmt.Errorf("repository: criar vendedor: %w", err)
	}
	return v, nil
}

// ─── FindByID ─────────────────────────────────────────────────────────────────

// FindByID retorna o vendedor pelo UUID. Retorna ErrVendorNotFound se ausente.
func (r *PGVendorRepository) FindByID(ctx context.Context, id string) (Vendor, error) {
	const q = `
		SELECT id, name, email, status, anonymized_at, created_at
		FROM vendors
		WHERE id = $1
	`
	var v Vendor
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&v.ID, &v.Name, &v.Email, &v.Status, &v.AnonymizedAt, &v.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Vendor{}, ErrVendorNotFound
		}
		return Vendor{}, fmt.Errorf("repository: buscar vendedor %q: %w", id, err)
	}
	return v, nil
}

// ─── List ─────────────────────────────────────────────────────────────────────

// List retorna vendedores com filtro opcional por status.
// Bind params em todos os filtros — CHK020 owasp.
func (r *PGVendorRepository) List(ctx context.Context, filter VendorFilter) ([]Vendor, error) {
	var (
		q    string
		args []any
	)
	if filter.Status != nil {
		q = `
			SELECT id, name, email, status, anonymized_at, created_at
			FROM vendors
			WHERE status = $1
			ORDER BY created_at DESC
		`
		args = []any{*filter.Status}
	} else {
		q = `
			SELECT id, name, email, status, anonymized_at, created_at
			FROM vendors
			ORDER BY created_at DESC
		`
	}

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: listar vendedores: %w", err)
	}
	defer rows.Close()

	var vendors []Vendor
	for rows.Next() {
		var v Vendor
		if err := rows.Scan(&v.ID, &v.Name, &v.Email, &v.Status, &v.AnonymizedAt, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("repository: scan vendedor: %w", err)
		}
		vendors = append(vendors, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: listar vendedores (rows): %w", err)
	}
	return vendors, nil
}

// ─── Update ───────────────────────────────────────────────────────────────────

// Update aplica o patch ao vendedor. Allowlist explícita de campos: name, email, status.
// Retorna ErrVendorNotFound se o vendedor não existir após o update.
// Retorna ErrVendorEmailConflict se o novo email já estiver em uso.
func (r *PGVendorRepository) Update(ctx context.Context, id string, patch VendorPatch) (Vendor, error) {
	if patch.Name == nil && patch.Email == nil && patch.Status == nil {
		return r.FindByID(ctx, id)
	}

	// Construção dinâmica com allowlist explícita (CHK020 — apenas campos permitidos).
	set := []string{}
	args := []any{}
	argN := 1

	if patch.Name != nil {
		set = append(set, fmt.Sprintf("name = $%d", argN))
		args = append(args, *patch.Name)
		argN++
	}
	if patch.Email != nil {
		set = append(set, fmt.Sprintf("email = $%d", argN))
		args = append(args, *patch.Email)
		argN++
	}
	if patch.Status != nil {
		set = append(set, fmt.Sprintf("status = $%d", argN))
		args = append(args, *patch.Status)
		argN++
	}

	// ID como último argumento.
	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE vendors SET %s
		WHERE id = $%d
		RETURNING id, name, email, status, anonymized_at, created_at
	`, joinComma(set), argN)

	var v Vendor
	err := r.pool.QueryRow(ctx, q, args...).Scan(
		&v.ID, &v.Name, &v.Email, &v.Status, &v.AnonymizedAt, &v.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Vendor{}, ErrVendorNotFound
		}
		if isUniqueViolationMsg(err, "email") {
			return Vendor{}, ErrVendorEmailConflict
		}
		return Vendor{}, fmt.Errorf("repository: atualizar vendedor %q: %w", id, err)
	}
	return v, nil
}

// ─── Anonymize ────────────────────────────────────────────────────────────────

// Anonymize apaga PII do vendedor e registra em audit_trail (CHK077/CHK078 LGPD).
// Formato de anonimização: name → "REMOVED_<uuid>", email → "removed_<sha256[:8]>@anon.invalid"
// A operação é atômica: anonimização + audit_trail em uma única transação.
func (r *PGVendorRepository) Anonymize(ctx context.Context, id string, actorID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("repository: iniciar tx anonymize: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Verificar que o vendedor existe e não foi anonimizado.
	var currentStatus VendorStatus
	var anonymizedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT status, anonymized_at FROM vendors WHERE id = $1 FOR UPDATE`, id).
		Scan(&currentStatus, &anonymizedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrVendorNotFound
		}
		return fmt.Errorf("repository: buscar vendedor para anonimizar: %w", err)
	}
	if anonymizedAt != nil {
		return fmt.Errorf("repository: vendedor %q já foi anonimizado em %s", id, anonymizedAt.Format(time.RFC3339))
	}

	// Anonimizar: name → "REMOVED_<id>", email → "removed_<id[:8]>@anon.invalid"
	anonName := "REMOVED_" + id
	anonEmail := "removed_" + id[:8] + "@anon.invalid"
	now := time.Now().UTC()

	_, err = tx.Exec(ctx, `
		UPDATE vendors
		SET name = $1, email = $2, anonymized_at = $3
		WHERE id = $4
	`, anonName, anonEmail, now, id)
	if err != nil {
		return fmt.Errorf("repository: anonimizar vendedor %q: %w", id, err)
	}

	// Registrar em audit_trail (CHK078 — entity_type='vendor_anonymization').
	// actor_user_id é UUID FK NULLABLE — usar NULL se actorID não for UUID válido.
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_trail (entity_type, entity_id, actor_user_id, from_state, to_state, reason)
		VALUES ('vendor_anonymization', $1, $2::uuid, $3, 'anonymized', 'LGPD deletion request')
	`, id, actorID, string(currentStatus))
	if err != nil {
		return fmt.Errorf("repository: audit_trail anonymize: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("repository: commit anonymize: %w", err)
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func joinComma(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}

// isUniqueViolation verifica se é um erro de violação UNIQUE pelo nome da constraint.
func isUniqueViolation(err error, constraintName string) bool {
	return err != nil && contains(err.Error(), constraintName) && contains(err.Error(), "unique")
}

// isUniqueViolationMsg verifica se o erro menciona unique constraint com a coluna.
func isUniqueViolationMsg(err error, field string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "unique") && contains(msg, field)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
