import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Button from 'components/Button';
import Modal from 'components/Modal';

import './CreateEnvironment.scss';

class CreateEnvironment extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  handleCloseClick() {
    this.close();
  }

  open() {
    this.dialog && this.dialog.open();
  }

  close() {
    this.dialog && this.dialog.close();
  }

  render() {
    return (
      <Modal 
        className="CreateEnvironment"
        ref={ref => this.dialog = ref} 
        title="New environment"
        width={540}
        offsetTop={8}>
        <div className="create-dialog-content layout vertical">
          {this.props.children}
          <div style={{ height: '20px' }} />
          <div className="layout horizontal-reverse center">
            <Button 
              color="grey"
              solid
              secondary
              onClick={this.handleCloseClick}>
              Close
            </Button>
          </div>
        </div>
      </Modal>
    );
  }
}

CreateEnvironment.propTypes = {
  dispatch: PropTypes.func.isRequired,
  children: PropTypes.node.isRequired,
  space: PropTypes.object.isRequired
};

export default ReactRedux.connect(() => {
  return {};
}, null, null, { withRef: true })(CreateEnvironment);