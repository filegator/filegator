// https://docs.cypress.io/api/introduction/api.html
Cypress.Cookies.defaults({
  whitelist: 'filegator' // do not clear this cookie every time
})

describe('Browser', () => {

  // once before any of the tests run.
  before(function () {
  })

  // before each test
  beforeEach(function () {
    cy.server()
    cy.route('GET', '?r=/getconfig', 'fixture:getconfig')
    cy.route('GET', '?r=/getuser', 'fixture:getuser')
    cy.route('POST', '?r=/getdir', 'fixture:dir_home')
    cy.route('POST', '?r=/changedir', 'fixture:dir_mydocs')
    cy.route('GET', '?r=/listusers', 'fixture:listusers')
  })

  it('Visits the root', () => {
    cy.visit('/')
    cy.contains('My Documents')
  })

  it('Preview & edit txt file', () => {
    cy.visit('/')
    cy.contains('read_only_demo.txt')
    cy.get('.dropdown').last().click()
    cy.get('.fa-file-alt').last().click()
    cy.contains('Close')
    cy.contains('Save')
  })

  it('Go to subfolder', () => {
    cy.visit('/')
    cy.contains('My Documents').click()
    cy.contains('Cool Project.doc')
    cy.contains('Home')
  })

  it('Login admin', function() {
    cy.viewport(1024, 768)
    cy.route('POST', '?r=/login', 'fixture:login_admin')
    cy.visit('/')
    cy.contains('Log in').click()
    cy.get('input[name="username"]').type('admin')
    cy.get('input[type="password"]').type('admin123')
    cy.contains('Log in').click()

    cy.contains('a.navbar-item', 'Files')
    cy.contains('a.navbar-item', 'Users')
    cy.contains('a.navbar-item', 'Log out')
    cy.contains('a.navbar-item', 'Admin').click()
    cy.contains('.modal', 'Old password')
    cy.contains('.modal', 'New password')
    cy.get('.modal-close').click()
    cy.contains('a.navbar-item', 'Users').click()
    cy.contains('admin')
    cy.contains('guest')
    cy.contains('john').click()
    cy.contains('.modal', 'Role')
    cy.contains('.modal', 'Username')
    cy.contains('.modal', 'Name')
    cy.contains('.modal', 'Password')
    cy.contains('.modal', 'Home Folder')
    cy.contains('.modal', 'Permissions')
    cy.contains('.modal .field', 'Read')
    cy.contains('.modal .field', 'Write')
    cy.contains('.modal .field', 'Upload')
    cy.contains('.modal .field', 'Download')
    cy.contains('.modal .field', 'Batch Download')
    cy.contains('.modal .field', 'Zip')
    cy.get('.modal-close').click()
  })

  it('Check multiple files', () => {
    cy.viewport(1024, 768)
    cy.visit('/')
    cy.contains('Selected: 0 of 9')
    cy.contains('#multi-actions', 'Add files')
    cy.contains('#multi-actions', 'New')
    cy.get('span.check').first().click()
    cy.contains('Selected: 9 of 9')
    cy.contains('#multi-actions', 'Download')
    cy.contains('#multi-actions', 'Copy')
    cy.contains('#multi-actions', 'Move')
    cy.contains('#multi-actions', 'Zip')
    cy.contains('#multi-actions', 'Delete')
  })

  it('Single dir actions', () => {
    cy.viewport(1024, 768)
    cy.visit('/')
    cy.contains('tr.type-dir', 'Copy')
    cy.contains('tr.type-dir', 'Move')
    cy.contains('tr.type-dir', 'Rename')
    cy.contains('tr.type-dir', 'Zip')
    cy.contains('tr.type-dir', 'Delete')
  })

  it('Single file actions', () => {
    cy.viewport(1024, 768)
    cy.visit('/')
    cy.contains('tr.type-file', 'Download')
    cy.contains('tr.type-file', 'Copy')
    cy.contains('tr.type-file', 'Move')
    cy.contains('tr.type-file', 'Rename')
    cy.contains('tr.type-file', 'Zip')
    cy.contains('tr.type-file', 'Unzip')
    cy.contains('tr.type-file', 'Delete')
    cy.contains('tr.type-file', 'Copy link')

    // click on single file action button
    cy.get('.dropdown-trigger').last().click()
    cy.get('.dropdown-content').last().contains('Download').should('be.visible')
    // close context menu
    cy.get('.dropdown-trigger').last().click()
    cy.get('.dropdown-content').last().contains('Download').should('not.be.visible')

    // right-click on the file row should also open context menu
    cy.get('tr.type-file').last().rightclick()
    cy.get('.dropdown-content').last().contains('Download').should('be.visible')
    // close context menu with another right-click
    cy.get('tr.type-file').last().rightclick()
    cy.get('.dropdown-content').last().contains('Download').should('not.be.visible')
  })

  it('New folder and file', () => {
    cy.visit('/')
    cy.contains('New').click()
    cy.contains('.dropdown-content', 'Folder')
    cy.contains('.dropdown-content', 'File')
  })

  it('Tree view', () => {
    cy.viewport(1024, 768)
    cy.visit('/')
    cy.contains('My Documents').click()
    cy.get('#sitemap').click()
    cy.contains('Projects')
    cy.contains('Test')
    cy.contains('Close').click()
  })

  it('Search', () => {
    cy.viewport(1024, 768)
    cy.visit('/')
    cy.contains('My Documents').click()
    cy.get('#search').click()
    cy.contains('Search')
    cy.contains('Name')
    cy.contains('Close').click()
  })

})
