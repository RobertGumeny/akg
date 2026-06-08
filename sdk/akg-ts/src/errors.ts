/** Thrown when a deleteNode or deleteEdge target, or a required node reference, does not exist. */
export class NotFoundError extends Error {
  override name = 'NotFoundError';
  constructor(message = 'not found') {
    super(message);
  }
}

/** Thrown when an argument violates a format or semantic constraint (e.g. invalid type/tag/relation name, deleting a node with live edges, negative limit, or a closed store). */
export class InvalidInputError extends Error {
  override name = 'InvalidInputError';
  constructor(message = 'invalid input') {
    super(message);
  }
}

/** Thrown when a required field is structurally absent — a putNode without a title, or a decoded file record missing a required field. */
export class MissingRequiredFieldError extends Error {
  override name = 'MissingRequiredFieldError';
  constructor(message = 'missing required field') {
    super(message);
  }
}
