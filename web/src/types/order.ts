// Tipos de Pedido — espelha backend/internal/dto/order.go
// Ref: contracts/api.md §/orders; spec §FR-008 (máquina de estado)

export type OrderStatus = 'rascunho' | 'confirmado' | 'pago' | 'cancelado';

export interface OrderItem {
  id: string; // UUID
  description: string; // máx 500 chars (CHK019)
  quantity: number; // inteiro positivo
  unitPriceCents: number; // int64 centavos
  lineTotalCents: number; // int64 centavos
}

export interface Order {
  id: string; // UUID
  vendorId: string;
  totalCents: number; // int64 centavos (P-III — sem float)
  orderDate: string; // ISO 8601 date (YYYY-MM-DD)
  status: OrderStatus;
  paidAt: string | null; // ISO 8601 UTC
  items: OrderItem[];
}

export interface OrderCreate {
  vendorId: string;
  orderDate: string;
  items: Omit<OrderItem, 'id' | 'lineTotalCents'>[];
}
