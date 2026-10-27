/**
 * Small DOM helpers.
 */

/** Returns the element with the given id, throwing when missing. */
export function el(id) {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error(`missing element #${id}`);
  }
  return element;
}

/** Shows an element. */
export function show(element) {
  element.classList.remove("hidden");
}

/** Hides an element. */
export function hide(element) {
  element.classList.add("hidden");
}

/** Sets visibility from a boolean. */
export function setVisible(element, visible) {
  element.classList.toggle("hidden", !visible);
}

/** Creates an element with attributes and children. */
export function create(tag, { className, text, attrs } = {}, children = []) {
  const node = document.createElement(tag);
  if (className) {
    node.className = className;
  }
  if (text !== undefined) {
    node.textContent = text;
  }
  if (attrs) {
    for (const [name, value] of Object.entries(attrs)) {
      if (value !== undefined && value !== null && value !== false) {
        node.setAttribute(name, value === true ? "" : value);
      }
    }
  }
  for (const child of children) {
    node.append(child);
  }
  return node;
}

/** Removes all children of an element. */
export function clear(element) {
  element.replaceChildren();
}
