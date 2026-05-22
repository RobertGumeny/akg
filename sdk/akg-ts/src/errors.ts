export class NotFoundError extends Error {
  override name = 'NotFoundError';
  constructor(message = 'not found') {
    super(message);
  }
}

export class InvalidInputError extends Error {
  override name = 'InvalidInputError';
  constructor(message = 'invalid input') {
    super(message);
  }
}

export class MissingRequiredFieldError extends Error {
  override name = 'MissingRequiredFieldError';
  constructor(message = 'missing required field') {
    super(message);
  }
}
