# MiniBankGo - A Modern Banking API Service

MiniBankGo is a robust banking API service built with Go (Golang) that provides secure and efficient banking operations including user management, transactions, and KYC verification.

## Features

- User Registration and Authentication
- KYC (Know Your Customer) Management
- Transaction Processing (Deposit, Withdraw, Transfer)
- Admin Dashboard and User Management
- Audit Logging
- Rate Limiting and Security Features
- Real-time Balance Updates
- Transaction History
- AML (Anti-Money Laundering) Compliance

## Prerequisites

- Go 1.21 or higher
- SQLite (for database)
- Environment variables setup

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/minibank-go.git
cd minibank-go
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file with the following variables:
```
DATABASE_URL=minibank.db
JWT_SECRET=your-secret-key-change-in-production
ENCRYPTION_KEY=MiniBankGo2025SecureKey123456789
ADMIN_CODE=MINIBANK_ADMIN_2025
PORT=8080
ENVIRONMENT=development
```

4. Run the application:
```bash
go run main.go
```

## API Endpoints

### Authentication

- `POST /api/register` - Register a new user
- `POST /api/login` - Authenticate user
- `GET /api/health` - Health check endpoint

### Transactions

- `POST /api/transactions/deposit` - Deposit money
- `POST /api/transactions/withdraw` - Withdraw money
- `POST /api/transactions/transfer` - Transfer money between users
- `GET /api/transactions` - View transaction history

### KYC Management

- `POST /api/kyc/submit` - Submit KYC documents
- `POST /api/admin/kyc/verify` - Verify KYC (Admin only)

### Admin Operations

- `GET /api/admin/users` - List all users (Admin only)
- `GET /api/admin/audit-logs` - View audit logs (Admin only)

## Security Features

- JWT-based Authentication
- Rate limiting
- Input validation
- Secure password hashing
- AML compliance checks
- Daily transaction limits
- IP address tracking
- Audit logging

## Configuration

The application can be configured using environment variables:

- `DATABASE_URL`: Database connection string
- `JWT_SECRET`: JWT signing secret
- `ENCRYPTION_KEY`: Key for sensitive data encryption
- `ADMIN_CODE`: Code for admin registration
- `PORT`: Server port
- `ENVIRONMENT`: Application environment (development/production)
- `MAX_TRANSFER_AMOUNT`: Maximum transfer amount
- `DAILY_TRANSFER_LIMIT`: Daily transfer limit

## Error Handling

The API returns standardized error responses with appropriate HTTP status codes:

- `400 Bad Request`: Invalid input or validation errors
- `401 Unauthorized`: Authentication required
- `403 Forbidden`: Permission denied
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server errors

## Testing

The project includes comprehensive test cases for all major functionality. Here are the detailed test cases:

### Authentication Tests

1. **Health Check**
   - Status code is 200
   - Response has status "healthy"

2. **User Registration**
   - Status code is 201
   - User created successfully message
   - Stores user ID in collection variables

3. **Admin Registration**
   - Status code is 201
   - Admin user created successfully message
   - Verifies admin status and admin privileges

4. **User Login**
   - Status code is 200
   - Verifies token received
   - Stores JWT token in collection variables

5. **Admin Login**
   - Status code is 200
   - Verifies admin token received
   - Stores admin token in collection variables

### User Profile Tests

1. **Get Profile**
   - Status code is 200
   - Verifies profile data received
   - Checks for required fields (email, first_name)

### Transaction Tests

1. **Deposit Money**
   - Status code is 201
   - Verifies deposit successful message
   - Checks new balance
   - Validates transaction record

2. **Withdraw Money**
   - Status code is 201
   - Verifies withdrawal successful message
   - Checks new balance
   - Validates transaction record

3. **Transfer Money**
   - Status code is 201
   - Verifies transfer successful message
   - Checks sender's new balance
   - Validates both sender and recipient transactions
   - Verifies reference uniqueness

4. **View Transaction History**
   - Status code is 200
   - Verifies transaction list returned
   - Checks pagination
   - Validates transaction details

### KYC Tests

1. **Submit KYC**
   - Status code is 201
   - Verifies KYC submitted message
   - Checks KYC ID returned
   - Validates KYC status

2. **Verify KYC (Admin)**
   - Status code is 200
   - Verifies KYC verification updated
   - Checks status change
   - Validates audit log entry

### Error Handling Tests

1. **Invalid Credentials**
   - Status code is 401
   - Verifies error message
   - Checks unauthorized access

2. **Insufficient Balance**
   - Status code is 400
   - Verifies error message
   - Checks balance validation

3. **Invalid Amount**
   - Status code is 400
   - Verifies validation errors
   - Checks amount constraints

4. **Duplicate Registration**
   - Status code is 400
   - Verifies error message
   - Checks unique constraints

5. **Unauthorized Access**
   - Status code is 403
   - Verifies permission denied
   - Checks admin access requirements

### Security Tests

1. **JWT Validation**
   - Verifies token validation
   - Checks token expiration
   - Validates token claims

2. **Rate Limiting**
   - Tests request limits
   - Verifies rate limit responses
   - Checks window reset

3. **Input Validation**
   - Tests required fields
   - Validates field formats
   - Checks length constraints

4. **Transaction Limits**
   - Tests daily limits
   - Verifies AML rules
   - Checks maximum amounts

5. **Audit Logging**
   - Verifies log creation
   - Checks log details
   - Validates log format

### Environment Variables

The following environment variables are used for testing:

```bash
base_url=http://localhost:8080
jwt_token=  # Stores user JWT token
user_id=    # Stores registered user ID
admin_token= # Stores admin JWT token
```

### Test Scripts

The test scripts use Postman's test framework to:
- Verify HTTP status codes
- Validate response JSON structure
- Check error messages
- Store and reuse variables
- Test edge cases
- Validate security constraints
- Check database consistency

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details

## Support

For support, please open an issue in the GitHub repository or contact the development team.
