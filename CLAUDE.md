# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

FileGator is a multi-user file manager web application built with PHP backend and Vue.js frontend. It provides file operations through a REST API with role-based access control and supports multiple storage adapters via Flysystem.

## Common Development Commands

### Development Setup
```bash
# Local development with PHP and Node.js
cp configuration_sample.php configuration.php
chmod -R 775 private/
chmod -R 775 repository/
composer install --ignore-platform-reqs
npm install
npm run build
npm run serve
```

### Docker Development
```bash
docker compose -f docker-compose-dev.yml up
```

### Build and Serve
- `npm run serve` - Start development server with PHP backend and Vue frontend
- `npm run build` - Build frontend for production
- `npm run lint` - Lint Vue.js frontend code

### Testing
- `vendor/bin/phpunit` - Run PHP unit and feature tests
- `vendor/bin/phpstan analyse ./backend` - Static analysis for PHP backend
- `npm run test:unit` - Run Vue.js unit tests 
- `npm run test:e2e` - Run Cypress end-to-end tests

## Architecture

### Backend (PHP)
- **Framework**: Custom PHP framework with dependency injection container
- **Entry Point**: `index.php` loads configuration and bootstraps the `App` class
- **Configuration**: `configuration.php` defines services, adapters, and frontend config
- **Routing**: FastRoute-based routing defined in `backend/Controllers/routes.php`
- **Services**: Adapter pattern for auth, storage, logging, sessions, etc.
- **Controllers**: Handle HTTP requests for file operations, auth, and admin functions
- **Namespace**: All backend code uses `Filegator\` namespace with PSR-4 autoloading

### Frontend (Vue.js 2)
- **Framework**: Vue.js 2 with Vuex, Vue Router, Buefy (Bulma-based components)
- **Entry Point**: `frontend/main.js` initializes Vue app and configures axios
- **Views**: Browser (file manager), Login, Users (admin panel)
- **API**: Axios-based API client in `frontend/api/api.js`
- **Build**: Vue CLI with custom webpack config in `vue.config.js`

### Key Architectural Patterns
- **Service Layer**: Services implement interfaces with adapter pattern (Auth, Storage, Logger, etc.)
- **Dependency Injection**: Container-based DI for service resolution
- **Role-Based Access**: Routes define required roles and permissions
- **Configuration-Driven**: Services configured via `configuration.php` arrays

### Service Adapters
- **Auth**: JsonFile, Database, LDAP, WordPress, RESTAuth
- **Storage**: Local filesystem via Flysystem (extensible to S3, FTP, etc.)
- **Session**: Symfony session handlers (file, database, Redis, etc.)
- **Logger**: Monolog with configurable handlers

### File Structure
- `backend/` - PHP application code
- `frontend/` - Vue.js application
- `dist/` - Built frontend assets (production deployment root)
- `private/` - Non-web-accessible files (logs, sessions, user data)
- `repository/` - Default file storage location
- `tests/` - PHPUnit and Cypress tests

## Development Notes

### Configuration
- Copy `configuration_sample.php` to `configuration.php` for local development
- Services are configured as arrays with handler classes and config options
- Frontend configuration is embedded in backend config under `frontend_config`

### Authentication & Authorization
- User roles: guest, user, admin
- Permissions: read, write, upload, download, batchdownload, zip, chmod
- RESTAuth adapter recently added for external authentication systems

### Testing Requirements
- PHP extensions needed: xdebug, php-zip, sqlite
- Cypress tests require running development server
- PHPUnit tests include both unit and feature tests

### Build Process
- Vue CLI builds frontend to `dist/` directory
- Production deployment should serve from `dist/` as document root
- Frontend communicates with backend via query parameter routing (`?r=endpoint`)