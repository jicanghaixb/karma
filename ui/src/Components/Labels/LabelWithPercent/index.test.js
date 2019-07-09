import React from "react";

import { mount } from "enzyme";

import toDiffableHtml from "diffable-html";

import { AlertStore, NewUnappliedFilter } from "Stores/AlertStore";

import { LabelWithPercent } from ".";

let alertStore;

beforeEach(() => {
  alertStore = new AlertStore([]);
});

const MountedLabelWithPercent = (name, value) => {
  return mount(
    <LabelWithPercent
      alertStore={alertStore}
      name={name}
      value={value}
      hits={25}
      percent={50}
    />
  ).find(".components-label");
};

const RenderAndClick = (name, value, clickOptions) => {
  const tree = MountedLabelWithPercent(name, value);
  tree.find(".components-label").simulate("click", clickOptions || {});
};

describe("<MountedLabelWithPercent />", () => {
  it("matches snapshot", () => {
    const tree = mount(
      <LabelWithPercent
        alertStore={alertStore}
        name="foo"
        value="bar"
        hits={25}
        percent={50}
      />
    );
    expect(toDiffableHtml(tree.html())).toMatchSnapshot();
  });

  it("calling onClick() adds a new filter 'foo=bar'", () => {
    RenderAndClick("foo", "bar");
    expect(alertStore.filters.values).toHaveLength(1);
    expect(alertStore.filters.values).toContainEqual(
      NewUnappliedFilter("foo=bar")
    );
  });

  it("calling onClick() while holding Alt key adds a new filter 'foo!=bar'", () => {
    RenderAndClick("foo", "bar", { altKey: true });
    expect(alertStore.filters.values).toHaveLength(1);
    expect(alertStore.filters.values).toContainEqual(
      NewUnappliedFilter("foo!=bar")
    );
  });
});
