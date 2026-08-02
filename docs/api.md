# API HTTP

Base: `/api/v1`. Corpos e respostas usam JSON.

| Método | Rota | Descrição |
|---|---|---|
| GET | `/services` | Lista serviços |
| POST | `/services` | Cria serviço |
| GET | `/professionals` | Lista profissionais |
| POST | `/professionals` | Cria profissional |
| GET | `/customers` | Lista clientes |
| POST | `/customers` | Cria cliente |
| GET | `/appointments?from=&to=` | Lista agenda no período |
| POST | `/appointments` | Cria agendamento |
| PATCH | `/appointments/{id}/status` | Altera estado |

Exemplo de agendamento:

```json
{
  "customer_id": "uuid",
  "professional_id": "uuid",
  "service_id": "uuid",
  "starts_at": "2026-08-01T14:00:00-03:00",
  "notes": "Preferência por máquina 1"
}
```

Erros seguem o formato:

```json
{ "error": { "code": "schedule_conflict", "message": "..." } }
```
