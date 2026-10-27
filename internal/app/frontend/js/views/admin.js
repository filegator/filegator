/**
 * Administrator pages: storage root picker, user management and settings.
 */

import { api, ApiError } from "../modules/api.js";
import { clear, create, el, hide, show } from "../modules/dom.js";
import { confirmAction, dialogRoot } from "../modules/dialog.js";
import { DEFAULT_DATE_FORMAT, formatDatePattern } from "../modules/datetime.js";
import { t } from "../modules/i18n.js";
import { BUILT_IN_LOGO, BUILT_IN_LOGO_DARK } from "../modules/logo.js";

const ROLES = ["user", "admin", "guest"];
const PERMISSIONS = [
  "read",
  "write",
  "upload",
  "download",
  "batchdownload",
  "zip",
  "chmod",
];

/** The only permissions a guest account can hold (mirrors auth.GuestPermissions). */
const GUEST_PERMISSIONS = ["read", "download", "batchdownload"];

/** Human label for a role. */
function roleLabel(role) {
  return role.charAt(0).toUpperCase() + role.slice(1);
}

/**
 * Creates the admin view.
 *
 * callbacks:
 *   onRootChange(path) - storage root changed, refresh the browser
 *   onClose()          - leave the admin view
 */
export function createAdmin({ onRootChange, onClose, onSettingsChange, capabilities = {} }) {
  const view = el("admin-view");
  // The strip is .admin-nav; the buttons go in the .admin-tabs row inside it.
  const tabs = el("admin-tabs");
  const panel = el("admin-panel");
  const toast = el("admin-toast");

  let activeTab = "users";
  let settingsLanguages = [];
  let toastTimer = null;

  function showToast(message, kind = "error") {
    toast.textContent = message;
    toast.classList.toggle("is-info", kind === "info");
    show(toast);
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => hide(toast), 4000);
  }

  function reportError(err) {
    if (err instanceof ApiError && err.status === 401) {
      onClose(true);
      return;
    }
    showToast(err instanceof ApiError ? err.message : "Unknown error");
  }

  // ---------------------------------------------------------------- storage

  async function renderStorage() {
    clear(panel);
    // No "Loading" line: the fetch is local and the form appears in its place
    // a moment later, so the placeholder only ever flashed.

    let current;
    try {
      current = await api.adminStorageConfig();
    } catch (err) {
      reportError(err);
      return;
    }

    const form = create("form", { className: "settings-form" }, []);
    const error = create("p", { className: "form-error hidden" });

    const type = create("select", { className: "select" });
    for (const [value, label] of [
      ["local", "Local folder"],
      ["sftp", "SFTP"],
      ["s3", "S3"],
    ]) {
      type.append(
        Object.assign(document.createElement("option"), {
          value,
          textContent: label,
          selected: (current.type || "local") === value,
        })
      );
    }
    form.append(labelWrap("Storage type", type));

    // Local backend: a folder on this machine.
    const localBlock = create("div", {}, []);
    const rootInput = create("input", {
      className: "input",
      attrs: { type: "text", value: current.type === "local" || !current.type ? current.root || "" : "repository", spellcheck: "false" },
    });
    const pick = button("Select Folder", async () => {
      const chosen = await chooseRoot(rootInput.value || "/");
      if (chosen) {
        rootInput.value = chosen;
      }
    }, "ghost");
    localBlock.append(
      labelWrap("Folder", create("div", { className: "input-with-button" }, [rootInput, pick]))
    );

    // SFTP backend: a remote server.
    const sftpBlock = create("div", {}, []);
    const host = field(sftpBlock, "Host", "text", current.sftp_host || "");
    const port = field(sftpBlock, "Port", "number", current.sftp_port || 22);
    const user = field(sftpBlock, "Username", "text", current.sftp_user || "");
    const password = field(sftpBlock, "Password", "password", "");
    if (current.sftp_has_password) {
      password.placeholder = "Leave blank to keep the current password";
    }
    const keyFile = field(sftpBlock, "Private key file", "text", current.sftp_private_key || "");
    const knownHosts = field(sftpBlock, "known_hosts", "text", current.sftp_known_hosts || "");
    const remoteRoot = field(
      sftpBlock,
      "Remote folder",
      "text",
      current.type === "sftp" ? current.root || "/" : "/"
    );
    const timeout = field(sftpBlock, "Timeout (seconds)", "number", current.sftp_timeout || 15);

    // S3 backend: a bucket, credentials and an optional endpoint.
    const s3Block = create("div", {}, []);
    const bucket = field(s3Block, "Bucket", "text", current.s3_bucket || "");
    const region = field(s3Block, "Region", "text", current.s3_region || "us-east-1");
    const s3Endpoint = field(s3Block, "Endpoint", "text", current.s3_endpoint || "");
    s3Endpoint.placeholder = "https://nyc3.digitaloceanspaces.com";
    const accessKey = field(s3Block, "Access key", "text", current.s3_access_key || "");
    const secretKey = field(s3Block, "Secret key", "password", "");
    if (current.s3_has_secret) {
      secretKey.placeholder = "Leave blank to keep the current password";
    }
    const prefix = field(s3Block, "Prefix", "text", current.s3_prefix || "");
    const pathStyle = create("input", { className: "checkbox", attrs: { type: "checkbox" } });
    pathStyle.checked = Boolean(current.s3_use_path_style);
    s3Block.append(labelWrap("Path-style URLs (MinIO, some S3 services)", pathStyle, "inline"));
    const s3Timeout = field(s3Block, "Timeout (seconds)", "number", current.s3_timeout || 60);

    function syncBlocks() {
      const isLocal = type.value === "local";
      localBlock.classList.toggle("hidden", !isLocal);
      sftpBlock.classList.toggle("hidden", type.value !== "sftp");
      s3Block.classList.toggle("hidden", type.value !== "s3");
    }
    type.addEventListener("change", syncBlocks);

    form.append(localBlock, sftpBlock, s3Block, error);
    form.append(
      create("div", { className: "dialog-actions" }, [
        button("Save", null, "primary", "submit"),
      ])
    );

    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      error.classList.add("hidden");

      const isLocal = type.value === "local";
      const payload = {
        type: type.value,
        root: isLocal ? rootInput.value.trim() : type.value === "sftp" ? remoteRoot.value.trim() : "",
        sftp_host: host.value.trim(),
        sftp_port: Number(port.value) || 22,
        sftp_user: user.value.trim(),
        sftp_password: password.value,
        sftp_has_password: Boolean(current.sftp_has_password),
        sftp_private_key: keyFile.value.trim(),
        sftp_known_hosts: knownHosts.value.trim(),
        sftp_timeout: Number(timeout.value) || 15,
        s3_bucket: bucket.value.trim(),
        s3_region: region.value.trim(),
        s3_endpoint: s3Endpoint.value.trim(),
        s3_access_key: accessKey.value.trim(),
        s3_secret_key: secretKey.value,
        s3_has_secret: Boolean(current.s3_has_secret),
        s3_use_path_style: pathStyle.checked,
        s3_prefix: prefix.value.trim(),
        s3_timeout: Number(s3Timeout.value) || 0,
      };

      try {
        await api.adminSaveStorageConfig(payload);
        showToast("Done", "info");
        onRootChange();
        renderStorage();
      } catch (err) {
        error.textContent = err instanceof ApiError ? err.message : "Unknown error";
        error.classList.remove("hidden");
      }
    });

    const summary = (() => {
      if (current.type === "sftp") {
        return `sftp://${current.sftp_host || "?"}${current.root || "/"}`;
      }
      if (current.type === "s3") {
        return `s3://${current.s3_bucket || "?"}${current.s3_prefix ? "/" + current.s3_prefix : ""}`;
      }
      return current.root;
    })();

    panel.append(
      create("h2", { className: "admin-heading", text: "Repository" }),
      create("p", {
        className: "admin-hint",
        text: "Files are managed in this location. Switching applies immediately.",
      }),
      create("p", { className: "admin-current", text: summary }),
      form
    );

    syncBlocks();
  }

  /**
   * Opens a folder browser and returns the chosen path, or null.
   *
   * options.scoped - when true the browser only walks the storage root and
   * cannot go above it. Used for home folders; the storage root picker needs
   * the whole server.
   */
  async function chooseRoot(startPath, options = {}) {
    const scoped = Boolean(options.scoped);
    const browse = scoped ? api.adminBrowseStorage : api.adminBrowse;
    // Both modes treat "/" as the top; scoped mode simply cannot reach past
    // the storage root because the server refuses those paths.
    const atTop = (dir) => dir === "/";
    let path = startPath;
    let finish = () => { };

    // The picker gets its own overlay stacked over any dialog already open
    // (for example the user form), so closing it restores that form instead of
    // wiping it.
    const pathLabel = create("p", { className: "picker-path", text: startPath });
    const body = create("div", { className: "picker-body" }, []);

    // The footer is built once. "Use this folder" reads the live path, so it
    // never closes over a stale value.
    const dialog = create("div", { className: "dialog picker-dialog" }, [
      closeHeader("Choose a folder", () => close(null)),
      pathLabel,
      body,
      create("div", { className: "dialog-actions" }, [
        button("Cancel", () => close(null)),
        button("Use this folder", () => close(path), "primary"),
      ]),
    ]);
    const root = create("div", {
      className: "dialog-root picker-root",
      attrs: { role: "dialog", "aria-modal": "true" },
    });
    document.body.append(root);

    // close is used by the buttons above, so it must exist before they are
    // clicked; finish is wired to the promise resolver below.
    function close(value) {
      root.remove();
      finish(value);
    }

    return new Promise((resolve) => {
      finish = resolve;

      async function render(dir) {
        let result;
        try {
          result = await browse(dir);
        } catch (err) {
          showToast("Cannot read this folder");
          return;
        }
        path = result.path;

        // Only the body is rebuilt when navigating; the header and footer stay
        // put so their handlers never multiply or hold a stale path.
        clear(body);
        pathLabel.textContent = path;

        const list = create("ul", { className: "picker-list" });
        body.append(list);
        // In scoped mode the storage root is the top: no ".." there.
        if (!atTop(path)) {
          const parent = create("li", {}, [
            create("button", { className: "picker-item", text: "..", attrs: { type: "button" } }),
          ]);
          parent.firstChild.addEventListener("click", () => {
            const up = path.replace(/\/+$/, "").split("/").slice(0, -1).join("/") || "/";
            render(up);
          });
          list.append(parent);
        }
        for (const dir of result.directories) {
          const item = create("li", {}, [
            create("button", { className: "picker-item", text: `📁 ${dir.name}`, attrs: { type: "button" } }),
          ]);
          item.firstChild.addEventListener("click", () => render(dir.path));
          list.append(item);
        }
        if (!result.directories.length) {
          list.append(create("li", { className: "picker-empty", text: "No subfolders" }));
        }
      }

      root.onkeydown = (event) => {
        if (event.key === "Escape") {
          event.preventDefault();
          close(null);
        }
      };
      root.onclick = (event) => {
        if (event.target === root) {
          close(null);
        }
      };
      root.replaceChildren(dialog);
      root.classList.remove("hidden");
      render(path);
    });
  }

  async function changeRoot() {
    const current = await api.adminRoot().catch(() => ({ path: "/" }));
    const chosen = await chooseRoot(current.path);
    if (!chosen) {
      return;
    }
    try {
      await api.adminSetRoot(chosen);
      showToast("Done", "info");
      await renderStorage();
      onRootChange(chosen);
    } catch (err) {
      reportError(err);
    }
  }

  // ------------------------------------------------------------------ users

  async function renderUsers() {
    clear(panel);
    let users;
    try {
      users = await api.adminUsers();
    } catch (err) {
      reportError(err);
      return;
    }

    const table = create("table", { className: "admin-table" }, [
      create("thead", {}, [
        create("tr", {}, [
          create("th", { text: "Username" }),
          create("th", { text: "Name" }),
          create("th", { text: "Role" }),
          create("th", { text: "Home" }),
          // Its own class: the heading is pushed to the right edge so it sits
          // over the buttons it labels.
          create("th", { className: "col-actions", text: "Actions" }),
        ]),
      ]),
    ]);
    const manageable = capabilities.auth_manage_users !== false;
    const body = create("tbody");
    // The guest is listed last: it publishes the repository, so it should not
    // sit among ordinary accounts where it could be enabled by accident.
    const ordered = [
      ...users.filter((u) => u.role !== "guest"),
      ...users.filter((u) => u.role === "guest"),
    ];
    for (const user of ordered) {
      const isGuest = user.role === "guest";
      const publicNow = isGuest && (user.permissions || []).includes("read");
      const rowActions = manageable
        ? isGuest
          // A guest is switched on and off rather than deleted: deleting
          // would silently unpublish the repository and lose its folder.
          ? [
            button(publicNow ? "Disable" : "Enable", () => toggleGuest(user), "ghost"),
            button("Edit", () => editUser(user), "ghost"),
          ]
          // Set password first: a password nobody can recover is the most
          // common reason to call an administrator, so it is the action
          // reached for most often and gets its own button instead of a trip
          // through the whole user form. Delete stays last and apart, where a
          // destructive action belongs.
          : [
            button("Set password", () => setPassword(user), "ghost"),
            button("Edit", () => editUser(user), "ghost"),
            button("Delete", () => deleteUser(user), "ghost danger"),
          ]
        : [];
      const actions = create("td", { className: "col-actions" }, [
        create("div", { className: "admin-row-actions" }, rowActions),
      ]);
      body.append(
        create("tr", { className: isGuest ? "is-guest" : "" }, [
          create("td", {}, [
            create("span", { text: user.username }),
            isGuest
              ? create("span", {
                className: publicNow ? "admin-badge is-public" : "admin-badge",
                text: publicNow ? "Public" : "Off",
              })
              : "",
          ]),
          create("td", { text: user.name || "" }),
          create("td", { text: roleLabel(user.role) }),
          create("td", { text: user.homedir }),
          actions,
        ])
      );
    }
    table.append(body);

    panel.append(
      create("div", { className: "admin-row-spread" }, [
        create("h2", { className: "admin-heading", text: "Users" }),
        ...(manageable
          ? [button("New user", () => editUser(null), "primary")]
          : [create("span", {
            className: "admin-hint",
            text: "This authentication provider does not manage users.",
          })]),
      ]),
      table
    );
  }

  /** Opens the create/edit form. */
  async function editUser(user, options = {}) {
    const editing = user !== null;
    let home = user ? user.homedir : "/";

    const form = create("form", { className: "user-form" }, []);
    const error = create("p", { className: "form-error hidden" });

    const username = field(form, "Username", "text", user ? user.username : "");
    const name = field(form, "Name", "text", user ? user.name : "");
    const password = field(
      form,
      editing ? "New password" : "Password",
      "password",
      ""
    );
    if (editing) {
      password.placeholder = "Leave blank to keep the current password";
    }
    // Say what the field is for, directly under it. When self-service password
    // changes are switched off this is the only way to help a user who cannot
    // sign in, and nothing else in the interface says so.
    if (editing) {
      password.setAttribute("aria-describedby", "user-password-hint");
      form.append(
        create("p", {
          className: "admin-hint",
          attrs: { id: "user-password-hint" },
          text: "Administrators can set a new password for a user who cannot sign in.",
        })
      );
    }

    const role = create("select", { className: "select" });
    for (const r of ROLES) {
      role.append(
        Object.assign(document.createElement("option"), {
          value: r,
          textContent: roleLabel(r),
          selected: user ? user.role === r : r === "user",
        })
      );
    }
    form.append(labelWrap("Role", role));

    // Home folder with a "browse" button.
    const homeInput = create("input", {
      className: "input",
      attrs: { type: "text", value: home, readonly: "" },
    });
    const browse = button("Select Folder", async () => {
      // Home folders must stay inside the storage root, so the picker starts
      // there and cannot climb above it. The server rejects anything else.
      const chosen = await chooseRoot(home || "/", { scoped: true });
      if (chosen) {
        home = chosen;
        homeInput.value = chosen;
      }
    }, "ghost");
    form.append(labelWrap("Home", create("div", { className: "input-with-button" }, [homeInput, browse])));

    const createHome = create("input", {
      className: "checkbox",
      attrs: { type: "checkbox", checked: editing ? "" : "" },
    });
    form.append(labelWrap("Create home folder if it does not exist", createHome));

    // Permissions. A guest can only ever read and download, so the rest are
    // not offered: a checkbox that silently does nothing is worse than none.
    const permBoxes = new Map();
    const perms = create("div", { className: "perm-grid" });
    const permitted = () => (role.value === "guest" ? GUEST_PERMISSIONS : PERMISSIONS);
    const renderPerms = () => {
      permBoxes.clear();
      perms.replaceChildren();
      for (const p of permitted()) {
        const box = create("input", { className: "checkbox", attrs: { type: "checkbox" } });
        if (user && user.permissions.includes(p)) {
          box.checked = true;
        }
        permBoxes.set(p, box);
        // Checkbox first, then its name, so the boxes form a column the eye
        // can run down. Each is its own label: wrapping the whole grid in one
        // label made every click toggle whichever control came first.
        perms.append(
          create("label", { className: "perm-option" }, [
            box,
            create("span", { className: "perm-option-label", text: roleLabel(p) }),
          ])
        );
      }
    };
    renderPerms();
    // The group needs a heading of its own, not a <label>: a label around a
    // container links to nothing and fights the grid's layout.
    const permsField = create("div", { className: "field" }, [
      create("span", { className: "field-label", attrs: { id: "user-perms-label" }, text: "Permissions" }),
      perms,
    ]);
    form.append(permsField);
    const permsHint = create("p", { className: "admin-hint hidden" });
    form.insertBefore(permsHint, perms.nextSibling);
    // Switching the role changes which permissions exist, so redraw and
    // explain why the list shrank.
    role.addEventListener("change", () => {
      renderPerms();
      const isGuest = role.value === "guest";
      permsHint.classList.toggle("hidden", !isGuest);
      permsHint.textContent = isGuest
        ? "A guest can only read, download and batch-download. Clear them to switch guest access off."
        : "";
    });

    form.append(error, create("div", { className: "dialog-actions" }, [
      button("Cancel", () => closeDialogRoot(), "ghost"),
      button(editing ? "Save" : "Create", null, "primary", "submit"),
    ]));

    const root = dialogRoot();
    root.onkeydown = (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeDialogRoot();
      }
    };

    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      error.classList.add("hidden");

      const payload = {
        username: username.value.trim(),
        name: name.value.trim(),
        role: role.value,
        homedir: home,
        permissions: [...permBoxes.entries()].filter(([, box]) => box.checked).map(([p]) => p),
        create_home: createHome.checked,
      };
      if (password.value) {
        payload.password = password.value;
      }

      try {
        if (editing) {
          const target = encodeURIComponent(user.username);
          await api.adminUpdateUser(target, payload);
        } else {
          await api.adminCreateUser(payload);
        }
        closeDialogRoot();
        showToast("Done", "info");
        renderUsers();
      } catch (err) {
        error.textContent = err instanceof ApiError ? err.message : "Unknown error";
        error.classList.remove("hidden");
      }
    });

    const dialog = create("div", { className: "dialog user-dialog" }, [
      closeHeader(editing ? "Edit user" : "New user", closeDialogRoot),
      form,
    ]);
    // Tapping the backdrop closes the dialog, as with every other dialog.
    root.onclick = (event) => {
      if (event.target === root) {
        closeDialogRoot();
      }
    };
    root.replaceChildren(dialog);
    root.classList.remove("hidden");
    if (options.focusPassword) {
      password.focus();
    } else {
      username.focus();
    }
  }

  /**
   * Opens the "Set password" dialog for one user.
   *
   * Only the password is sent: the endpoint leaves the rest of the account
   * alone, so this cannot disturb the role, home folder or permissions.
   */
  function setPassword(user) {
    const root = dialogRoot();
    const restoreFocus = document.activeElement;
    const form = create("form", { className: "user-form" });
    const error = create("p", { className: "form-error hidden" });

    const password = create("input", {
      className: "input",
      attrs: {
        type: "password",
        autocomplete: "new-password",
        required: "required",
        "aria-describedby": "set-password-hint",
      },
    });
    form.append(labelWrap("New password", password));
    form.append(
      create("p", {
        className: "admin-hint",
        attrs: { id: "set-password-hint" },
      })
    );
    form.append(error);
    form.append(
      create("div", { className: "dialog-actions" }, [
        button("Cancel", () => close(), "ghost"),
        button("Save", null, "primary", "submit"),
      ])
    );

    const close = () => {
      closeDialogRoot();
      if (restoreFocus && document.contains(restoreFocus)) {
        restoreFocus.focus();
      }
    };

    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      const value = password.value;
      if (!value) {
        error.textContent = "Password is required";
        error.classList.remove("hidden");
        return;
      }
      try {
        await api.adminSetPassword(encodeURIComponent(user.username), value);
        close();
        showToast("Done", "info");
      } catch (err) {
        error.textContent = err instanceof ApiError ? err.message : "Unknown error";
        error.classList.remove("hidden");
      }
    });

    const dialog = create("div", { className: "dialog user-dialog" }, [
      closeHeader(`Set password for ${user.username}`, close),
      form,
    ]);
    root.onclick = (event) => {
      if (event.target === root) {
        close();
      }
    };
    // Escape closes, like every other dialog: touch devices have no key, but
    // the × and the backdrop cover those.
    root.onkeydown = (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        close();
      }
    };
    root.replaceChildren(dialog);
    root.classList.remove("hidden");
    password.focus();
  }

  function closeDialogRoot() {
    const root = dialogRoot();
    root.classList.add("hidden");
    root.onclick = null;
    root.replaceChildren();
  }

  /**
   * Turns guest access on or off with one click.
   *
   * "On" grants exactly the permissions a guest can hold, read and download:
   * a guest that may list a folder but open nothing is useless. "Off" clears
   * them, which is what makes anonymous visitors get the login screen.
   * Needs an existing home folder, so a missing one is reported instead of
   * silently publishing nothing.
   */
  async function toggleGuest(user) {
    const enable = !(user.permissions || []).includes("read");
    try {
      await api.adminUpdateUser(encodeURIComponent(user.username), {
        username: user.username,
        name: user.name,
        role: "guest",
        homedir: user.homedir,
        permissions: enable ? [...GUEST_PERMISSIONS] : [],
        create_home: false,
      });
      showToast(enable ? "Guest access is on" : "Guest access is off", "info");
      renderUsers();
    } catch (err) {
      showToast(err instanceof ApiError ? err.message : "Unknown error");
    }
  }

  async function deleteUser(user) {
    const confirmed = await confirmAction({
      title: "Delete",
      message: `Are you sure you want to do this? (${user.username})`,
      confirmText: "Delete",
      danger: true,
    });
    if (!confirmed) {
      return;
    }
    try {
      await api.adminDeleteUser(encodeURIComponent(user.username));
      showToast("Done", "info");
      renderUsers();
    } catch (err) {
      reportError(err);
    }
  }


  // ------------------------------------------------------------------ auth

  async function renderAuth() {
    clear(panel);
    let current;
    try {
      current = await api.adminAuthConfig();
    } catch (err) {
      reportError(err);
      return;
    }

    const form = create("form", { className: "settings-form" }, []);
    const error = create("p", { className: "form-error hidden" });

    const type = create("select", { className: "select" });
    for (const [value, label] of [
      ["json", "Users file (local)"],
      ["ldap", "LDAP"],
      ["mysql", "MySQL"],
      ["postgres", "PostgreSQL"],
    ]) {
      type.append(
        Object.assign(document.createElement("option"), {
          value,
          textContent: label,
          selected: (current.type || "json") === value,
        })
      );
    }
    form.append(labelWrap("Authentication", type));

    // Local users file.
    const jsonBlock = create("div", {}, []);
    const usersFile = field(jsonBlock, "Users file", "text", current.users_file || "");

    // LDAP directory.
    const ldapBlock = create("div", {}, []);
    const server = field(ldapBlock, "Server", "text", current.ldap_server || "");
    server.placeholder = "ldaps://dc.example.com:636";
    const bindDN = field(ldapBlock, "Bind DN", "text", current.ldap_bind_dn || "");
    bindDN.placeholder = "cn=service,dc=example,dc=com";
    const bindPass = field(ldapBlock, "Bind password", "password", "");
    if (current.ldap_has_bind_pass) {
      bindPass.placeholder = "Leave blank to keep the current password";
    }
    const baseDN = field(ldapBlock, "Base DN", "text", current.ldap_base_dn || "");
    baseDN.placeholder = "cn=users,dc=example,dc=com";
    const filter = field(ldapBlock, "Filter", "text", current.ldap_filter || "(uid=*)");
    const usernameAttr = field(ldapBlock, "Username attribute", "text", current.ldap_username_attribute || "uid");
    const nameAttr = field(ldapBlock, "Name attribute", "text", current.ldap_name_attribute || "cn");
    const addDomain = field(ldapBlock, "Add domain to username", "text", current.ldap_add_domain || "");
    addDomain.placeholder = "@example.com";
    const homeTemplate = field(ldapBlock, "Home folder template", "text", current.ldap_home_template || "");
    homeTemplate.placeholder = "/users/%s";
    const adminNames = field(
      ldapBlock,
      "Admin usernames",
      "text",
      (current.ldap_admin_usernames || []).join(", ")
    );
    const cacheTTL = field(ldapBlock, "User cache (seconds)", "number", current.ldap_user_cache_ttl || 60);
    const timeout = field(ldapBlock, "Timeout (seconds)", "number", current.ldap_timeout || 10);

    // Permissions granted to every directory user.
    const permBoxes = new Map();
    const permGrid = create("div", { className: "perm-grid" });
    for (const p of ["read", "write", "upload", "download", "batchdownload", "zip", "chmod"]) {
      const box = create("input", { className: "checkbox", attrs: { type: "checkbox" } });
      box.checked = (current.ldap_default_permissions || []).includes(p);
      permBoxes.set(p, box);
      permGrid.append(labelWrap(roleLabel(p), box, "inline"));
    }
    ldapBlock.append(labelWrap("Default permissions", permGrid));

    // SQL backend: a database table with the same columns as users.json.
    const sqlBlock = create("div", {}, []);
    const sqlDSN = field(sqlBlock, "Connection string (DSN)", "password", "");
    if (current.sql_has_dsn) {
      sqlDSN.placeholder = "Leave blank to keep the current password";
    }
    sqlDSN.placeholder = current.sql_has_dsn
      ? "Leave blank to keep the current password"
      : "user:pass@tcp(host:3306)/filegator";
    const sqlTable = field(sqlBlock, "Users table", "text", current.sql_table || "users");
    const sqlCache = field(sqlBlock, "User cache (seconds)", "number", current.sql_user_cache_ttl || 60);
    const sqlMaxOpen = field(sqlBlock, "Max open connections", "number", current.sql_max_open_conns || 10);
    const sqlMaxIdle = field(sqlBlock, "Max idle connections", "number", current.sql_max_idle_conns || 5);
    const sqlTimeout = field(sqlBlock, "Timeout (seconds)", "number", current.sql_timeout || 10);
    sqlBlock.append(
      create("p", {
        className: "admin-hint",
        text: "The table must already exist; see the schema in the documentation.",
      })
    );

    function syncBlocks() {
      jsonBlock.classList.toggle("hidden", type.value !== "json");
      ldapBlock.classList.toggle("hidden", type.value !== "ldap");
      sqlBlock.classList.toggle("hidden", type.value !== "mysql" && type.value !== "postgres");
    }
    type.addEventListener("change", syncBlocks);

    form.append(jsonBlock, ldapBlock, sqlBlock, error);
    form.append(
      create("div", { className: "dialog-actions" }, [
        button("Save", null, "primary", "submit"),
      ])
    );

    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      error.classList.add("hidden");

      const payload = {
        type: type.value,
        users_file: usersFile.value.trim(),
        ldap_server: server.value.trim(),
        ldap_bind_dn: bindDN.value.trim(),
        ldap_bind_pass: bindPass.value,
        ldap_has_bind_pass: Boolean(current.ldap_has_bind_pass),
        ldap_base_dn: baseDN.value.trim(),
        ldap_filter: filter.value.trim(),
        ldap_username_attribute: usernameAttr.value.trim(),
        ldap_name_attribute: nameAttr.value.trim(),
        ldap_add_domain: addDomain.value.trim(),
        ldap_remove_domains: current.ldap_remove_domains || [],
        ldap_attributes: current.ldap_attributes || [],
        ldap_default_permissions: [...permBoxes.entries()]
          .filter(([, box]) => box.checked)
          .map(([p]) => p),
        ldap_admin_usernames: adminNames.value
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
        ldap_home_template: homeTemplate.value.trim(),
        ldap_user_cache_ttl: Number(cacheTTL.value) || 0,
        ldap_timeout: Number(timeout.value) || 0,
        sql_dsn: sqlDSN.value,
        sql_has_dsn: Boolean(current.sql_has_dsn),
        sql_table: sqlTable.value.trim(),
        sql_user_cache_ttl: Number(sqlCache.value) || 0,
        sql_max_open_conns: Number(sqlMaxOpen.value) || 0,
        sql_max_idle_conns: Number(sqlMaxIdle.value) || 0,
        sql_timeout: Number(sqlTimeout.value) || 0,
      };

      try {
        await api.adminSaveAuthConfig(payload);
        showToast("Done", "info");
        // Capabilities and the current session may change: reload the app.
        window.location.reload();
      } catch (err) {
        error.textContent = err instanceof ApiError ? err.message : "Unknown error";
        error.classList.remove("hidden");
      }
    });

    panel.append(
      create("h2", { className: "admin-heading", text: "Authentication" }),
      create("p", {
        className: "admin-hint",
        text: "Where user accounts come from. Switching takes effect immediately.",
      }),
      form
    );

    syncBlocks();
  }

  // --------------------------------------------------------------------- ui

  /**
   * Renders the UI section: how the interface looks and behaves for everyone.
   *
   * It saves only its own fields. The server keeps the settings this page does
   * not show, so a save here cannot reset the Files or Security pages (and the
   * reverse).
   */
  async function renderUI() {
    clear(panel);
    let settings;
    let status = {};
    try {
      settings = await api.adminSettings();
      // The interface source is only known to the server, and the form has to
      // say whether a saved folder is already in effect.
      status = (await api.adminStatus().catch(() => ({}))) || {};
    } catch (err) {
      reportError(err);
      return;
    }

    const form = create("form", { className: "settings-form" }, []);
    const error = create("p", { className: "form-error hidden" });

    const appName = field(form, "Title", "text", settings.app_name);

    // Language picker limited to the languages the server provides.
    const language = create("select", { className: "select" });
    const available = settingsLanguages.length ? settingsLanguages : [settings.language];
    for (const code of available) {
      language.append(
        Object.assign(document.createElement("option"), {
          value: code,
          textContent: code,
          selected: code === settings.language,
        })
      );
    }
    form.append(labelWrap("Default language", language));

    const languageSelection = checkbox(settings.language_selection);
    form.append(labelWrap("Allow users to change the language", languageSelection));

    // The logo pair. Each field is a URL; an empty one means the FileGator
    // artwork embedded in the binary, which is what a fresh install has. One
    // pair serves every surface — the browser header, the admin header and the
    // login card — so there is nothing else to configure for the login page.
    // The preview renders the pair on split light/dark halves so the
    // administrator sees which picture appears in each theme, not just what
    // the URL says.
    const logo = field(form, "Logo URL", "text", settings.logo || "");
    logo.placeholder = "Leave empty to use the built-in logo";
    const logoDark = field(form, "Logo URL (dark mode)", "text", settings.logo_dark || "");
    logoDark.placeholder = "Leave empty to use the built-in dark logo";
    const logoPreview = create("div", { className: "logo-preview" }, []);
    form.append(
      logoPreview,
      create("p", {
        className: "admin-hint",
        text: "Shown in the header and on the login page. An empty field means the built-in FileGator logo. A path starting with / is served by this application; an http(s) URL is loaded from elsewhere.",
      })
    );

    // The server substitutes the built-in artwork for an empty field, so an
    // empty field previews the picture the page will actually show.
    renderLogoPreview(logoPreview, logo, logoDark, BUILT_IN_LOGO, BUILT_IN_LOGO_DARK);
    // Date pattern, in the moment.js tokens v7 used so an upgraded config
    // keeps working. A preview renders the pattern with the current moment,
    // because "YY/MM/DD hh" is hard to read until it is shown as a date.
    const dateFormat = field(form, "Date format", "text", settings.date_format);
    dateFormat.placeholder = DEFAULT_DATE_FORMAT;
    const datePreview = create("p", { className: "admin-hint date-preview" });
    const renderDatePreview = () => {
      const pattern = dateFormat.value.trim() || DEFAULT_DATE_FORMAT;
      datePreview.textContent = `${"Preview"}: ${formatDatePattern(new Date(), pattern)}`;
    };
    dateFormat.addEventListener("input", renderDatePreview);
    renderDatePreview();
    form.append(
      datePreview,
      create("p", {
        className: "admin-hint",
        text: "Tokens: YYYY year, MM month, DD day, HH 24-hour, hh 12-hour, mm minutes, ss seconds, MMM month name, dddd weekday. Use [text] to print a word.",
      })
    );

    // Interface source. Applied at startup, so the hint says a restart is
    // needed: changing it silently doing nothing would be worse than saying so.
    const frontendDir = field(form, "Interface folder", "text", settings.frontend_dir || "");
    frontendDir.placeholder = "Leave empty to use the built-in interface";
    const frontendSource = status.frontend_source || "embedded";
    const activeDir = settings.frontend_dir || "";
    const inUse = frontendSource === "directory" ? activeDir : "";
    form.append(
      create("p", {
        className: "admin-hint",
        text: inUse
          ? `Currently served from: ${inUse}. Changes apply after a restart.`
          : "Using the built-in interface. A folder set here takes effect after a restart.",
      })
    );
    form.append(
      create("p", {
        className: "admin-hint",
        text: "Export the built-in interface with: filegator -export-frontend <folder>",
      })
    );
    // Sends logged-out visitors elsewhere instead of showing the login form.
    const guestRedirection = field(
      form,
      "Guest redirection URL",
      "text",
      settings.guest_redirection || ""
    );
    guestRedirection.placeholder = "https://example.com/login";
    form.append(
      create("p", {
        className: "admin-hint",
        text: "Visitors who are not logged in are sent to this URL instead of the login form.",
      })
    );

    // Uploads moved to the Files page, together with the archive name and the
    // editable extensions, so one page owns everything about file handling.
    // Interface width stays here: it is not about files.

    // Interface width. A file listing stretched across a very wide monitor is
    // hard to read; 0 removes the cap.
    const maxWidth = field(form, "Maximum interface width (px)", "number", settings.max_width ?? 960);
    maxWidth.min = "0";
    maxWidth.step = "1";
    form.append(create("p", { className: "admin-hint", text: "0 removes the limit." }));

    // The two account switches are here rather than on Security by decision
    // (2026-10-26): they control what the signed-in user's header offers, which
    // is what a visitor sees, not who may reach the server.

    // Self-service password changes. Off hides the profile button in the
    // header and makes the API refuse the request; administrators can still
    // set any user's password from the Users tab, which is how someone locked
    // out is helped when this is off.
    const allowPasswordChange = checkbox(settings.allow_password_change !== false);
    form.append(labelWrap("Users may change their own password", allowPasswordChange));

    // Logging out is meaningless behind SSO: the identity provider still holds
    // the session, so the button only looks broken. The session cookie still
    // expires on its own.
    const allowLogout = checkbox(settings.allow_logout !== false);
    form.append(labelWrap("Users may log out", allowLogout));

    // Everything else moved to the Security page: IP lists, the trusted
    // header, session lifetime, cookies, CSRF, lockout, the event log and
    // response headers.

    form.append(
      error,
      create("div", { className: "dialog-actions" }, [
        button("Save", null, "primary", "submit"),
      ])
    );

    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      error.classList.add("hidden");

      // Only the fields this page shows: the server keeps every other
      // setting as it is, so saving here cannot reset another page.
      const payload = {
        app_name: appName.value.trim(),
        language: language.value,
        language_selection: languageSelection.checked,
        logo: logo.value.trim(),
        logo_dark: logoDark.value.trim(),
        date_format: dateFormat.value.trim(),
        frontend_dir: frontendDir.value.trim(),
        guest_redirection: guestRedirection.value.trim(),
        // An empty field means "no limit", the same as typing 0: leaving it
        // blank is the least surprising reading of an empty number input.
        max_width: Math.max(0, Math.round(Number(maxWidth.value) || 0)),
        allow_password_change: allowPasswordChange.checked,
        allow_logout: allowLogout.checked,
      };

      try {
        const saved = await api.adminSaveSettings(payload);
        showToast("Done", "info");
        onSettingsChange(saved);
      } catch (err) {
        error.textContent = err instanceof ApiError ? err.message : "Unknown error";
        error.classList.remove("hidden");
      }
    });

    panel.append(
      create("h2", { className: "admin-heading", text: "UI" }),
      create("p", {
        className: "admin-hint",
        text: "How the interface looks and which files it shows. These apply to everyone using this server.",
      }),
      form
    );
  }

  // ------------------------------------------------------------------ files

  /**
   * Renders the Files section: everything about how files are uploaded,
   * archived and edited.
   *
   * It saves only its own fields. The server keeps the settings this page does
   * not show, so a save here cannot reset the Settings page (and the reverse).
   */
  async function renderFiles() {
    clear(panel);

    let settings;
    try {
      settings = await api.adminSettings();
    } catch (err) {
      reportError(err);
      return;
    }

    const form = create("form", { className: "settings-form" }, []);
    const error = create("p", { className: "form-error hidden" });

    const maxSize = field(form, "Max upload size (MB)", "number", mb(settings.upload_max_size));
    maxSize.min = "0";
    maxSize.placeholder = "0";
    const chunkSize = field(form, "Upload chunk size (KB)", "number", kb(settings.upload_chunk_size));
    chunkSize.min = "0";
    chunkSize.placeholder = "0";
    const simultaneous = field(form, "Simultaneous uploads", "number", settings.upload_simultaneous);
    simultaneous.min = "0";
    simultaneous.placeholder = "0";
    // 0 is "no limit" for the two sizes but not for the other two, so the hint
    // sits directly under the pair it describes rather than at the bottom.
    form.append(
      create("p", {
        className: "admin-hint",
        text: "0 in either size removes the limit. The other two must be at least 1.",
      })
    );

    const archiveName = field(form, "Default archive name", "text", settings.default_archive_name);
    archiveName.placeholder = "archive.zip";
    const editable = field(form, "Editable file extensions", "text", settings.editable.join(", "));
    editable.placeholder = ".txt, .css, .js, .html, .json, .md";
    form.append(
      create("p", {
        className: "admin-hint",
        text: "Comma separated. These files open in the editor instead of downloading.",
      })
    );

    // Extensions the browser renders instead of downloading. v7 kept this in
    // config.json as `download_inline` (default ["pdf"]); v8 widened the
    // default to the images the gallery shows, because a photo that arrives as
    // a download cannot be viewed.
    const inline = field(
      form,
      "Inline download extensions",
      "text",
      // An empty list is normal, not an error: it means "download everything".
      (settings.download_inline || []).join(", ")
    );
    inline.placeholder = "pdf, jpg, png";
    form.append(
      create("p", {
        className: "admin-hint",
        text: "Comma separated. These files open in the browser instead of downloading. * makes every file render.",
      })
    );
    // Which listed extensions can actually be script, so a save can say so.
    // Exact extension match: "index.html" is not rendered just because "html"
    // is listed, and neither is "notes.mhtml".
    const RISKY = ["svg", "html", "htm", "xml", "xsl", "mhtml", "shtml"];
    const riskyWarning = create("p", { className: "admin-hint is-warning hidden" });
    const renderRiskyWarning = () => {
      const risky = splitList(inline.value)
        .map((ext) => ext.replace(/^\./, "").toLowerCase())
        .filter((ext) => RISKY.includes(ext));
      if (!risky.length) {
        riskyWarning.classList.add("hidden");
        riskyWarning.textContent = "";
        return;
      }
      riskyWarning.textContent = `Warning: ${risky.join(", ")} can contain script, and rendering it shows it on this domain. Only do this for a folder you trust.`;
      riskyWarning.classList.remove("hidden");
    };
    inline.addEventListener("input", renderRiskyWarning);
    renderRiskyWarning();
    form.append(riskyWarning);

    form.append(
      error,
      create("div", { className: "dialog-actions" }, [
        button("Save", null, "primary", "submit"),
      ])
    );

    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      error.classList.add("hidden");

      // The sizes and the concurrency are validated by the server as positive
      // numbers, so a 0 here is reported back rather than silently saved.
      const payload = {
        upload_max_size: toBytes(maxSize.value, 1024 * 1024),
        upload_chunk_size: toBytes(chunkSize.value, 1024),
        upload_simultaneous: Number(simultaneous.value) || 0,
        default_archive_name: archiveName.value.trim(),
        editable: splitList(editable.value),
        download_inline: splitList(inline.value),
      };

      try {
        const saved = await api.adminSaveSettings(payload);
        showToast("Done", "info");
        onSettingsChange(saved);
      } catch (err) {
        error.textContent = err instanceof ApiError ? err.message : "Unknown error";
        error.classList.remove("hidden");
      }
    });

    panel.append(
      create("h2", { className: "admin-heading", text: "Files" }),
      create("p", {
        className: "admin-hint",
        text: "How files are uploaded, archived and edited.",
      }),
      form
    );
  }

  // --------------------------------------------------------------- security

  /**
   * Renders the Security section: who may reach the server, how sessions are
   * kept, what is logged and which headers are sent.
   *
   * It saves only these fields (the server keeps the rest). The two account
   * switches that change what the header offers live on the UI page.
   */
  async function renderSecurity() {
    clear(panel);

    let settings;
    try {
      settings = await api.adminSettings();
    } catch (err) {
      reportError(err);
      return;
    }

    const form = create("form", { className: "settings-form" }, []);
    const error = create("p", { className: "form-error hidden" });

    // Who may connect. A deny match always wins; an empty allowlist means
    // "everyone", which is why the hint says so.
    const allowlist = create("textarea", { className: "input textarea", attrs: { rows: "4" } });
    allowlist.value = settings.ip_allowlist.join("\n");
    form.append(labelWrap("IP allowlist", allowlist));

    const denylist = create("textarea", { className: "input textarea", attrs: { rows: "4" } });
    denylist.value = settings.ip_denylist.join("\n");
    form.append(labelWrap("IP denylist", denylist));
    form.append(
      create("p", {
        className: "admin-hint",
        text: "One IP address or CIDR range per line. An empty allowlist lets everyone in; a denylist entry always wins.",
      })
    );

    const trustedHeader = field(form, "Trusted IP header", "text", settings.trusted_ip_header);
    trustedHeader.placeholder = "CF-Connecting-IP";
    form.append(
      create("p", {
        className: "admin-hint",
        text: "Leave empty to use the connecting address only. Set this only behind a proxy you control.",
      })
    );

    // Sessions
    const sessionTTL = field(form, "Session lifetime (hours)", "number", Math.round(settings.session_ttl / 3600));
    const secureCookies = checkbox(settings.secure_cookies);
    form.append(labelWrap("Secure cookies (HTTPS only)", secureCookies));
    const csrf = checkbox(settings.csrf_protection);
    form.append(labelWrap("CSRF protection", csrf));

    // Lockout
    const lockoutAttempts = field(form, "Failed login attempts before lockout", "number", settings.lockout_attempts);
    const lockoutTimeout = field(form, "Lockout time (seconds)", "number", settings.lockout_timeout);

    // Event log: sign-ins, blocked addresses and file changes.
    const loggingEnabled = checkbox(settings.logging_enabled);
    form.append(labelWrap("Write an event log", loggingEnabled));
    const loggingFile = field(form, "Log file", "text", settings.logging_file);
    loggingFile.placeholder = "private/logs/app.log";
    const loggingLevel = create("select", { className: "select" });
    for (const level of ["debug", "info", "warn", "error"]) {
      loggingLevel.append(
        Object.assign(document.createElement("option"), {
          value: level,
          textContent: level,
          selected: level === settings.logging_level,
        })
      );
    }
    form.append(labelWrap("Log level", loggingLevel));
    const loggingMaxSize = field(
      form,
      "Maximum log size (MB)",
      "number",
      Math.round((settings.logging_max_size || 0) / (1024 * 1024))
    );
    form.append(
      create("p", {
        className: "admin-hint",
        text: "One previous file is kept. Set 0 to disable rotation.",
      })
    );

    // Response headers: name/value rows layered over the built-in defaults.
    const headers = headerEditor(settings.headers || {});
    form.append(
      labelWrap("Response headers", headers.element),
      create("p", {
        className: "admin-hint",
        text: "Added to every response. Leave the value empty to remove a default header.",
      })
    );

    form.append(
      error,
      create("div", { className: "dialog-actions" }, [
        button("Save", null, "primary", "submit"),
      ])
    );

    form.addEventListener("submit", async (event) => {
      event.preventDefault();
      error.classList.add("hidden");

      // Only the fields this page shows: the server keeps every other
      // setting as it is, so saving here cannot reset another page.
      const payload = {
        ip_allowlist: splitLines(allowlist.value),
        ip_denylist: splitLines(denylist.value),
        trusted_ip_header: trustedHeader.value.trim(),
        session_ttl: Math.round((Number(sessionTTL.value) || 0) * 3600),
        secure_cookies: secureCookies.checked,
        csrf_protection: csrf.checked,
        lockout_attempts: Number(lockoutAttempts.value) || 0,
        lockout_timeout: Number(lockoutTimeout.value) || 0,
        logging_enabled: loggingEnabled.checked,
        logging_file: loggingFile.value.trim(),
        logging_level: loggingLevel.value,
        logging_max_size: toBytes(loggingMaxSize.value, 1024 * 1024),
        headers: headers.values(),
      };

      try {
        const saved = await api.adminSaveSettings(payload);
        showToast("Done", "info");
        onSettingsChange(saved);
      } catch (err) {
        error.textContent = err instanceof ApiError ? err.message : "Unknown error";
        error.classList.remove("hidden");
      }
    });

    panel.append(
      create("h2", { className: "admin-heading", text: "Security" }),
      create("p", {
        className: "admin-hint",
        text: "Who may reach this server, how sessions are kept, and what is recorded.",
      }),
      form
    );
  }

  function checkbox(checked) {
    const box = create("input", { className: "checkbox", attrs: { type: "checkbox" } });
    box.checked = Boolean(checked);
    return box;
  }

  /** Converts a megabyte value to bytes. */
  function toBytes(value, unit) {
    return Math.max(0, Math.round(Number(value) || 0)) * unit;
  }

  /** Renders bytes as whole megabytes. */
  function mb(bytes) {
    return String(Math.round((bytes || 0) / (1024 * 1024)));
  }

  /** Renders bytes as whole kilobytes. */
  function kb(bytes) {
    return String(Math.round((bytes || 0) / 1024));
  }

  /** Splits a comma separated list, trimming and dropping empties. */
  function splitList(value) {
    return String(value || "")
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
  }

  /** Splits newline separated entries. */
  function splitLines(value) {
    return String(value || "")
      .split("\n")
      .map((item) => item.trim())
      .filter(Boolean);
  }


  /**
   * Builds an editable list of response headers.
   *
   * Returns { element, values() }, where values() yields the header map to
   * send. Rows with an empty name are ignored; an empty value is preserved
   * because it means "remove this default header".
   */
  function headerEditor(initial) {
    const list = create("div", { className: "header-rows" });
    const rows = [];

    function addRow(name, value) {
      const row = create("div", { className: "header-row" }, []);

      const nameInput = create("input", {
        className: "input",
        attrs: {
          type: "text",
          placeholder: "X-Frame-Options",
          value: name,
          spellcheck: "false",
          autocapitalize: "none",
        },
      });
      const valueInput = create("input", {
        className: "input",
        attrs: {
          type: "text",
          placeholder: "SAMEORIGIN",
          value: value,
          spellcheck: "false",
          autocapitalize: "none",
        },
      });
      const remove = create("button", {
        className: "button button-ghost button-icon",
        text: "×",
        attrs: { type: "button", title: "Delete", "aria-label": "Delete" },
      });
      remove.addEventListener("click", () => {
        const index = rows.indexOf(row);
        if (index >= 0) {
          rows.splice(index, 1);
        }
        row.remove();
      });

      row.append(nameInput, valueInput, remove);
      rows.push(row);
      list.append(row);
    }

    for (const [name, value] of Object.entries(initial)) {
      addRow(name, value);
    }

    const add = create("button", {
      className: "button",
      text: "Add header",
      attrs: { type: "button" },
    });
    add.addEventListener("click", () => {
      addRow("", "");
      const last = list.lastElementChild;
      if (last) {
        last.querySelector("input").focus();
      }
    });

    const element = create("div", { className: "header-editor" }, [list, add]);

    return {
      element,
      values() {
        const out = {};
        for (const row of rows) {
          const inputs = row.querySelectorAll("input");
          const name = inputs[0].value.trim();
          const value = inputs[1].value.trim();
          if (!name) {
            continue;
          }
          out[name] = value;
        }
        return out;
      },
    };
  }

  // ------------------------------------------------------------- rendering

  /**
   * Renders the version line shown above every tab.
   *
   * Two things matter to whoever runs this: which release the binary is, and
   * whether a customised interface belongs to it. A stale interface still
   * works, so this is a warning rather than an error.
   */
  function renderVersionStrip(status) {
    const strip = el("admin-version");
    clear(strip);
    if (!status || !status.version) {
      strip.classList.add("hidden");
      return;
    }
    strip.classList.remove("hidden");
    const line = create("span", { className: "admin-version-text", text: status.version });
    strip.append(line);

    if (status.frontend_source === "directory") {
      if (status.frontend_version_mismatch) {
        strip.append(
          create("span", {
            className: "admin-badge is-stale",
            text: `Interface exported for ${status.frontend_version}`,
            title: "Export it again with -export-frontend to match this build",
          })
        );
      } else if (status.frontend_version) {
        strip.append(
          create("span", { className: "admin-badge", text: "Custom interface" })
        );
      } else {
        strip.append(
          create("span", {
            className: "admin-badge is-stale",
            text: "Custom interface (unversioned)",
            title: "The folder has no version.json, so it cannot be checked",
          })
        );
      }
    }
  }

  function renderTabs() {
    clear(tabs);

    // Leaving the administration area is the first choice in the row: it is
    // another place to go, like the sections, not a header action.
    //
    // The arrow is markup, not part of the translation: it points left in a
    // left-to-right language and right in a right-to-left one, so direction
    // belongs to the document, and translating "Back" alone keeps every
    // language file free of punctuation that is not words.
    const back = button("Back", () => onClose && onClose(), "tab tab-back");
    const arrow = create("span", { className: "tab-arrow", "aria-hidden": "true" });
    arrow.textContent = "←";
    back.prepend(arrow, " ");
    back.setAttribute("role", "tab");
    back.setAttribute("aria-label", "Back to files");
    tabs.append(back);

    // The administration area is English only (user decision 2026-10-27), so
    // its labels are plain strings rather than t() keys. That is also why the
    // admin-only keys were removed from every language file: nothing outside
    // this file used them.
    const items = [
      ["users", "Users"],
      ["ui", "UI"],
      ["files", "Files"],
      ["security", "Security"],
      ["storage", "Repository"],
      ["auth", "Auth"],
    ];
    for (const [id, label] of items) {
      const tab = button(label, () => selectTab(id), id === activeTab ? "tab active" : "tab");
      tab.setAttribute("role", "tab");
      tabs.append(tab);
    }
  }

  function selectTab(id) {
    activeTab = id;
    renderTabs();
    // The version strip is small and tells an administrator what they are
    // running, so it is refreshed on every tab change.
    api.adminStatus()
      .then((status) => renderVersionStrip(status))
      .catch(() => { });
    if (id === "storage") {
      renderStorage();
    } else if (id === "ui") {
      renderUI();
    } else if (id === "files") {
      renderFiles();
    } else if (id === "security") {
      renderSecurity();
    } else if (id === "auth") {
      renderAuth();
    } else {
      renderUsers();
    }
  }

  return {
    show() {
      view.classList.remove("hidden");
      // Languages power the settings picker; load them once per open.
      api.languages()
        .then((info) => {
          settingsLanguages = info.languages || [];
        })
        .catch(() => {
          settingsLanguages = [];
        });
      selectTab(activeTab);
    },
    hide() {
      view.classList.add("hidden");
    },
    /** Opens the current administrator's form with the password focused. */
    async focusPassword() {
      let users;
      try {
        users = await api.adminUsers();
      } catch (err) {
        reportError(err);
        return;
      }
      const me = users.find((u) => u.role === "admin");
      if (me) {
        editUser(me, { focusPassword: true });
      }
    },
  };
}

// ---------------------------------------------------------------- helpers

/**
 * Builds a dialog header with a title and a visible close button.
 * Every dialog needs one so it can always be dismissed.
 */
function closeHeader(title, onClose) {
  const close = create("button", {
    className: "dialog-close",
    text: "×",
    attrs: { type: "button", "aria-label": "Cancel", title: "Cancel" },
  });
  close.addEventListener("click", onClose);
  return create("div", { className: "dialog-header" }, [
    create("h2", { className: "dialog-title", text: title }),
    close,
  ]);
}

/** Creates a button with an optional variant. */
function button(label, handler, variant = "", type = "button") {
  const cls = ["button"];
  if (variant.includes("primary")) cls.push("button-primary-solid");
  if (variant.includes("danger")) cls.push("button-danger");
  if (variant.includes("ghost")) cls.push("button-ghost");
  if (variant.includes("tab")) cls.push("tab");
  if (variant.includes("active")) cls.push("active");
  const node = create("button", { className: cls.join(" "), text: label, attrs: { type } });
  if (handler) {
    node.addEventListener("click", handler);
  }
  return node;
}

/** Wraps a control in a labelled field. */
function labelWrap(labelText, control, variant = "") {
  return create("label", { className: variant === "inline" ? "field field-check" : "field" }, [
    create("span", { className: "field-label", text: labelText }),
    control,
  ]);
}

/** Adds a text field to a form and returns the input. */
function field(form, labelText, type, value) {
  const input = create("input", {
    className: "input",
    attrs: { type, value, autocomplete: "off", spellcheck: "false" },
  });
  form.append(labelWrap(labelText, input));
  return input;
}

/**
 * Renders the light/dark pair a logo field will produce, side by side.
 *
 * The two halves are painted with their own backgrounds rather than the page's,
 * because the dark artwork is meant to be seen on a dark surface: on a light
 * admin page a white logo would look broken. An image that fails to load is
 * replaced by a "?" rather than a broken-image glyph, so a typo reads as
 * "missing" while the field stays editable.
 *
 * `fallbackLight`/`fallbackDark` are what the server substitutes for an empty
 * field — the built-in artwork — except that an empty dark field shows the
 * light logo when one was typed, which is the rule the server applies too.
 */
function renderLogoPreview(container, lightInput, darkInput, fallbackLight, fallbackDark) {
  const shownDark = () =>
    darkInput.value.trim() || lightInput.value.trim() || fallbackDark;

  function paint() {
    clear(container);
    const pair = [
      ["is-light", lightInput.value.trim() || fallbackLight, "Light mode"],
      ["is-dark", shownDark(), "Dark mode"],
    ];
    for (const [variant, src, label] of pair) {
      const img = create("img", { className: "logo-preview-image", attrs: { alt: "" } });
      img.addEventListener("error", () => img.classList.add("is-missing"));
      img.addEventListener("load", () => img.classList.remove("is-missing"));
      img.src = src;
      container.append(
        create("div", { className: `logo-preview-half ${variant}` }, [
          create("span", { className: "logo-preview-label", text: label }),
          img,
        ])
      );
    }
  }
  lightInput.addEventListener("input", paint);
  darkInput.addEventListener("input", paint);
  paint();
}

